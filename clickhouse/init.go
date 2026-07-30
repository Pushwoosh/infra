package infraclickhouse

import (
	"sync"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

// Init creates a container and connects every connection from the config in background.
// The caller must wait on the wait group before using the returned container.
// Connection failures are fatal.
func Init(wg *sync.WaitGroup, cfg ConnectionsConfig) *Container {
	cont := NewContainer()

	wg.Go(func() {
		for name, clickCfg := range cfg {
			if err := cont.Connect(name, clickCfg); err != nil {
				infralog.Fatal("connect to clickhouse", zap.String("connection", name), zap.Error(err))
			}
		}
	})

	return cont
}
