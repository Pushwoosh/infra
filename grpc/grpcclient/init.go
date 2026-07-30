package infragrpcclient

import (
	"sync"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Init creates a container and connects every connection from the config in background.
// The caller must wait on the wait group before using the returned container.
// Connection failures are fatal.
func Init(wg *sync.WaitGroup, cfg ConnectionsConfig, options ...grpc.DialOption) *Container {
	cont := NewContainer()

	wg.Go(func() {
		for name, grpcCfg := range cfg {
			if err := cont.Connect(name, grpcCfg, options...); err != nil {
				infralog.Fatal("connect to grpc", zap.String("connection", name), zap.Error(err))
			}
		}
	})

	return cont
}
