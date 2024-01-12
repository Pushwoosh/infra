package infragrpcserver

import (
	"context"
	"net"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Grpc struct {
	cfg *GrpcConfig

	srvOpts []grpc.ServerOption
	srv     *grpc.Server
}

// NewGrpc creates a new Grpc server
func NewGrpc(cfg *GrpcConfig, opts ...grpc.ServerOption) *Grpc {
	srv := grpc.NewServer(opts...)
	return &Grpc{
		cfg: cfg,

		srvOpts: opts,
		srv:     srv,
	}
}

func (s *Grpc) Registrar() grpc.ServiceRegistrar {
	return s.srv
}

// Start starts listening Grpc server and blocks until it exit.
func (s *Grpc) Start(_ context.Context) error {
	listener, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return errors.Wrap(err, "net.Listen")
	}

	reflection.Register(s.srv)

	go func() {
		infralog.Debug("serving gRPC", zap.String("address", s.cfg.Listen))
		if err := s.srv.Serve(listener); err != nil {
			infralog.Fatal("grpc server error", zap.Error(err))
		}
	}()

	// gRPC web
	if s.cfg.GrpcWeb == nil {
		return nil
	}

	grpcWebServer := grpcweb.WrapServer(s.srv,
		grpcweb.WithOriginFunc(func(origin string) bool { return true }))

	httpServer := &http.Server{Handler: grpcWebServer, Addr: s.cfg.GrpcWeb.Listen}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			infralog.Fatal("grpcweb server error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Grpc) Stop(ctx context.Context) error {
	serverStopped := make(chan struct{}, 1)
	go func() {
		s.srv.GracefulStop()
		serverStopped <- struct{}{}
	}()

	select {
	case <-serverStopped:
		return nil
	case <-ctx.Done():
		s.srv.Stop()
		return ctx.Err()
	}
}
