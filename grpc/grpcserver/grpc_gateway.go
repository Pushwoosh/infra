package infragrpcserver

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type GatewayStartupFunc func(gw *Gateway, endpoint string, dialOptions []grpc.DialOption)

type Gateway struct {
	cfg *GrpcGatewayConfig

	mux             *runtime.ServeMux
	srv             *http.Server
	startupFunc     GatewayStartupFunc
	grpcDialOptions []grpc.DialOption

	runningCtx       context.Context
	runningCtxCancel context.CancelFunc
}

// NewGateway creates a new Grpc Gateway server
func NewGateway(cfg *GrpcGatewayConfig, startupFunc GatewayStartupFunc, gwMuxOptions ...runtime.ServeMuxOption) *Gateway {
	runningCtx, runningCtxCancel := context.WithCancel(context.Background())

	grpcDialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if cfg.MaxCallRecvMsgSizeMB > 0 {
		grpcDialOptions = append(grpcDialOptions, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.MaxCallRecvMsgSizeMB*1024*1024)))
	}

	if cfg.MaxCallSendMsgSizeMB > 0 {
		grpcDialOptions = append(grpcDialOptions, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(cfg.MaxCallSendMsgSizeMB*1024*1024)))
	}

	return &Gateway{
		cfg: cfg,

		mux:             runtime.NewServeMux(gwMuxOptions...),
		startupFunc:     startupFunc,
		grpcDialOptions: grpcDialOptions,

		runningCtx:       runningCtx,
		runningCtxCancel: runningCtxCancel,
	}
}

func (s *Gateway) Mux() *runtime.ServeMux {
	return s.mux
}

func (s *Gateway) Context() context.Context {
	return s.runningCtx
}

// Start starts listening Grpc Gateway server and blocks until it exit.
func (s *Gateway) Start(_ context.Context) error {
	s.srv = &http.Server{
		Addr:    s.cfg.Listen,
		Handler: s.mux,
	}

	s.startupFunc(s, s.cfg.ForwardTo, s.grpcDialOptions)

	go func() {
		infralog.Debug("serving grpc gateway", zap.String("address", s.cfg.Listen))
		if err := s.srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}

			infralog.Fatal("grpc gateway server fatal error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Gateway) Stop(ctx context.Context) error {
	defer s.runningCtxCancel()
	return s.srv.Shutdown(ctx)
}

func DefaultMuxOptions() []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		MuxOptionCustomMarshaler(),
		MuxOptionCustomErrorHandler(),
	}
}

func MuxOptionCustomMarshaler() runtime.ServeMuxOption {
	jsonMarshalOpts := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	return runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: jsonMarshalOpts,
	})
}

func MuxOptionCustomErrorHandler() runtime.ServeMuxOption {
	return runtime.WithErrorHandler(func(
		ctx context.Context,
		mux *runtime.ServeMux,
		marshaler runtime.Marshaler,
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		if s, ok := status.FromError(err); ok && s.Code() == codes.ResourceExhausted {
			err = &runtime.HTTPStatusError{
				Err:        s.Err(),
				HTTPStatus: http.StatusServiceUnavailable,
			}
		}
		runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
	})
}
