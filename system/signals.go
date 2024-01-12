package infrasystem

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pushwoosh/infra/log"
	"github.com/pushwoosh/infra/operator"
	"go.uber.org/zap"
)

const defaultShutdownTimeout = time.Second * 60
const defaultShutdownGracePeriod = time.Second * 15

type Signals struct {
	mu        sync.RWMutex
	interrupt chan os.Signal       // signals from OS will arrive into this channel
	done      chan struct{}        // sending message to this channel will exit the Wait() loop
	handlers  map[os.Signal]func() // signal handlers
	ignore    []os.Signal          // signals that will be ignored

	shutdownTimeout     time.Duration
	shutdownGracePeriod time.Duration

	shutdownCallback func(ctx context.Context)

	op *infraoperator.Operator
}

// NewDefaultSignals creates signal controller with the following signals handlers:
//
//	SIGTERM, SIGINT, SIGQUIT - shut down program
//	SIGHUP - reload configuration
//	SIGUSR1 - ignored
func NewDefaultSignals(op *infraoperator.Operator, opts ...Option) *Signals {
	s := NewSignals(op, opts...)

	s.Add(syscall.SIGTERM, s.Shutdown)
	s.Add(syscall.SIGINT, s.Shutdown)
	s.Add(syscall.SIGQUIT, s.Shutdown)

	s.Add(syscall.SIGHUP, s.Reload)

	s.Ignore(syscall.SIGUSR1)

	return s
}

func NewSignals(op *infraoperator.Operator, opts ...Option) *Signals {
	ret := &Signals{
		// Package signal will not block sending to this channel.
		// We must ensure that channel has sufficient buffer space.
		// 1 is enough for non signal-heavy-loaded system.
		interrupt: make(chan os.Signal, 1),
		done:      make(chan struct{}, 1),
		handlers:  make(map[os.Signal]func()),
		op:        op,

		shutdownTimeout:     defaultShutdownTimeout,
		shutdownGracePeriod: defaultShutdownGracePeriod,
		shutdownCallback:    func(ctx context.Context) {},
	}

	for i := range opts {
		opts[i].apply(ret)
	}

	return ret
}

func (s *Signals) restartSignalling() {
	signal.Stop(s.interrupt)

	signals := make([]os.Signal, 0, len(s.handlers))
	for sig := range s.handlers {
		signals = append(signals, sig)
	}

	signal.Notify(s.interrupt, signals...)
	signal.Ignore(s.ignore...)
}

// Add - adds signal handler
func (s *Signals) Add(si os.Signal, handler func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.addSignalHandler(si, handler)
	s.removeSignalFromIgnoreList(si)

	s.restartSignalling()
}

// Remove - removes signal handler
func (s *Signals) Remove(si os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeSignalHandler(si)
	s.removeSignalFromIgnoreList(si)

	s.restartSignalling()
}

// Ignore - adds signal to signal ignore list
func (s *Signals) Ignore(si os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeSignalHandler(si)
	s.addSignalToIgnoreList(si)

	s.restartSignalling()
}

func (s *Signals) addSignalHandler(si os.Signal, handler func()) {
	s.handlers[si] = handler
}

func (s *Signals) removeSignalHandler(si os.Signal) {
	delete(s.handlers, si)
}

func (s *Signals) addSignalToIgnoreList(si os.Signal) {
	s.removeSignalFromIgnoreList(si)
	s.ignore = append(s.ignore, si)
}

func (s *Signals) removeSignalFromIgnoreList(si os.Signal) {
	if len(s.ignore) == 0 {
		return
	}

	for i := range s.ignore {
		if s.ignore[i] == si {
			s.ignore[i] = s.ignore[len(s.ignore)-1]
			s.ignore = s.ignore[:len(s.ignore)-1]
		}
	}
}

// Wait polls for signals
func (s *Signals) Wait() {
	for {
		select {
		case <-s.done:
			infralog.Debug("Exiting")
			return
		case si := <-s.interrupt:
			infralog.Info("Received signal", zap.String("signal", si.String()))

			s.mu.Lock()
			handler, ok := s.handlers[si]
			s.mu.Unlock()
			if !ok {
				continue
			}

			handler()
		}
	}
}

// Reload is a signal handler that reloads configuration
func (s *Signals) Reload() {
	panic("not implemented")
}

// Shutdown is a signal handler that initializes system shutdown
func (s *Signals) Shutdown() {
	infralog.Info("Shutting down")

	// https://learnk8s.io/graceful-shutdown
	// So you could use the first 15 seconds to continue operating as nothing happened.
	// Hopefully, the interval should be enough to propagate the endpoint removal to kube-proxy, Ingress controller, CoreDNS, etc.
	// And, as a consequence, less and less traffic will reach your Pod until it stops.
	// After the 15 seconds, it's safe to close your connection with the database (or any persistent connections) and terminate the process.
	if s.shutdownGracePeriod > 0 {
		infralog.Info("Sleep before shutdown")
		time.Sleep(s.shutdownGracePeriod)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)

	infralog.Info("Waiting for service to finish tasks")
	s.shutdownCallback(ctx)

	stopResults := s.op.StopAll(ctx)
	cancel()

	infralog.Info("StopAll")

	if len(stopResults) > 0 {
		for _, err := range stopResults {
			infralog.Error("Shutdown error", zap.Error(err))
		}
		return
	}

	s.done <- struct{}{}
}
