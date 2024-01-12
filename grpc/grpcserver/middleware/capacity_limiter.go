package inframiddleware

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerCapacityLimiterInterceptor returns a grpc server unary interceptor
// that limits amount of concurrently processed requests.
// serverName is a string that will be put into metrics label.
func UnaryServerCapacityLimiterInterceptor(serverName string, limit int) grpc.UnaryServerInterceptor {
	limiter := newLimiter(limit)
	metric := metricRequestsInProcess.WithLabelValues(serverName, "unary")

	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !limiter.Get() {
			return nil, status.Error(codes.ResourceExhausted, "the server has reached its concurrent requests limit")
		}
		metric.Inc()
		defer func() {
			metric.Dec()
			limiter.Put()
		}()
		return handler(ctx, req)
	}
}

// StreamServerCapacityLimiterInterceptor returns a grpc server stream interceptor
// that limits amount of concurrently processed requests.
// serverName is a string that will be put into metrics label.
func StreamServerCapacityLimiterInterceptor(serverName string, limit int) grpc.StreamServerInterceptor {
	limiter := newLimiter(limit)
	metric := metricRequestsInProcess.WithLabelValues(serverName, "stream")

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !limiter.Get() {
			return status.Error(codes.ResourceExhausted, "the server has reached its concurrent requests limit")
		}
		metric.Inc()
		defer func() {
			metric.Dec()
			limiter.Put()
		}()
		return handler(srv, ss)
	}
}

var metricRequestsInProcess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Subsystem: "server",
	Name:      "request_in_process",
	Help:      "The total number of requests that are currently being processed.",
}, []string{"server", "grpc_type"})

//nolint:gochecknoinits
func init() {
	prometheus.MustRegister(metricRequestsInProcess)
}

type capacityLimiter interface {
	// Get returns true if request can be processed and false if limit reached
	// should be called before server wants to process a request.
	Get() bool

	// Put signals capacity limiter that server has finished processing one request.
	Put()

	// Free returns the number of requests the server can accept.
	Free() int

	// Current returns the number of requests the server is currently processing.
	Current() int
}

func newLimiter(limit int) capacityLimiter {
	if limit > 0 {
		return newLimited(limit)
	} else {
		return newUnlimited()
	}
}

type limited struct {
	limit   int
	tickets chan struct{}
}

func newLimited(limit int) *limited {
	l := &limited{
		limit:   limit,
		tickets: make(chan struct{}, limit),
	}

	for i := 0; i < limit; i++ {
		l.tickets <- struct{}{}
	}

	return l
}

func (l *limited) Get() bool {
	select {
	case <-l.tickets:
		return true
	default:
		return false
	}
}

func (l *limited) Put() {
	l.tickets <- struct{}{}
}

func (l *limited) Free() int {
	return len(l.tickets)
}

func (l *limited) Current() int {
	return l.limit - l.Free()
}

type unlimited struct{}

func newUnlimited() *unlimited    { return &unlimited{} }
func (u *unlimited) Get() bool    { return true }
func (u *unlimited) Put()         {}
func (u *unlimited) Free() int    { return 0 }
func (u *unlimited) Current() int { return 0 }
