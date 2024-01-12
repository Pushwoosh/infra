package infrahttp

import (
	"context"
	"net"
	"net/http"

	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

type HTTP struct {
	cfg *Config

	server  *http.Server
	handler http.Handler
}

func NewHTTP(cfg *Config, handler http.Handler) *HTTP {
	return &HTTP{
		cfg:     cfg,
		handler: handler,
	}
}

func (srv *HTTP) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", srv.cfg.Listen)
	if err != nil {
		return errors.Wrap(err, "net.listen")
	}

	srv.server = &http.Server{
		Addr:    srv.cfg.Listen,
		Handler: srv.handler,
	}

	go func() {
		if err := srv.server.Serve(listener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				infralog.Fatal("http server error", zap.Error(err))
			}
		}
	}()

	return nil
}

func (srv *HTTP) Stop(ctx context.Context) error {
	return srv.server.Shutdown(ctx)
}
