package infrainfoserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pushwoosh/infra/log"
	"github.com/pushwoosh/infra/operator"
	"go.uber.org/zap"
)

// Info server is an internal http server that is used by k8s and prometheus to
// get live probes and metrics from the service.

type Info struct {
	cfg *Config
	op  *infraoperator.Operator

	mu  sync.Mutex
	srv *http.Server

	bp BuildParams
}

type BuildParams struct {
	AppName    string `json:"application"`
	AppVersion string `json:"version"`
	GitRev     string `json:"git_revision"`
	BuildDate  string `json:"build_date"`
}

var (
	_ infraoperator.Starter = (*Info)(nil)
	_ infraoperator.Stopper = (*Info)(nil)
	_ infraoperator.Checker = (*Info)(nil)
)

func NewInfo(
	cfg *Config,
	op *infraoperator.Operator,
	bp BuildParams,
) *Info {
	return &Info{
		cfg: cfg,
		op:  op,

		bp: bp,
	}
}

// Start starts listening info http server
func (s *Info) Start(_ context.Context) error {
	infralog.Info("Starting Info http server")

	mux := http.NewServeMux()
	mux.Handle("/healthz", s.HealthzHandler())
	mux.Handle("/readyz", s.ReadyzHandler())
	mux.Handle("/release", s.ReleaseHandler())
	mux.Handle("/metrics", promhttp.Handler())

	// pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.srv = &http.Server{
		Addr:    s.cfg.Listen,
		Handler: mux,
	}

	go func() {
		if err := s.srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}

			infralog.Fatal("info server error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Info) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.srv.Shutdown(ctx)
}

func (s *Info) Check(_ context.Context) error {
	return nil
}

func (s *Info) HealthzHandler() http.Handler {
	return s.commonHealthzAndReadyzHandler()
}

func (s *Info) ReadyzHandler() http.Handler {
	return s.commonHealthzAndReadyzHandler()
}

func (s *Info) commonHealthzAndReadyzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		if s.cfg.ChecksTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.ChecksTimeout)
			defer cancel()
		}

		checkErrors := s.op.Check(ctx)

		if len(checkErrors) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			return
		}

		for _, serviceError := range checkErrors {
			infralog.Error("Health check error", zap.Error(serviceError))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
	})
}

func (s *Info) ReleaseHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(s.bp)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
}
