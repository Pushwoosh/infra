package main

import (
	"context"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	infralog.Setup(infralog.DefaultConfig())

	ctx = infralog.WithField(ctx, zap.Int("key", 42))
	ctx = infralog.WithField(ctx, zap.Int("key", 69))
	ctx = infralog.WithFields(ctx,
		zap.Float64("pi", 3.14159265359),
		zap.Float64("e", 2.718281828459045))

	infralog.ErrorCtx(ctx, "error message")
}
