package infrasystem

import (
	"context"
	"time"
)

type Option interface {
	apply(s *Signals)
}

type optionWithShutdownTimeout time.Duration

func (o optionWithShutdownTimeout) apply(s *Signals) {
	s.shutdownTimeout = time.Duration(o)
}

func WithShutdownTimeout(timeout time.Duration) Option {
	return optionWithShutdownTimeout(timeout)
}

type optionWithShutdownGracePeriod time.Duration

func (o optionWithShutdownGracePeriod) apply(s *Signals) {
	s.shutdownGracePeriod = time.Duration(o)
}

func WithShutdownGracePeriod(timeout time.Duration) Option {
	return optionWithShutdownGracePeriod(timeout)
}

type optionWithShutdownCallback func(ctx context.Context)

func (o optionWithShutdownCallback) apply(s *Signals) {
	s.shutdownCallback = o
}

func WithShutdownCallback(fn func(ctx context.Context)) Option {
	return optionWithShutdownCallback(fn)
}
