package inframiddleware

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// UnaryServerLoggerInterceptor return a grpc server unary interceptor
// that puts given logger into request's context.
func UnaryServerLoggerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctxzap.ToContext(ctx, logger), req)
	}
}
