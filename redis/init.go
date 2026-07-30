package infraredis

import (
	"sync"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

// Init creates a container and connects every enabled connection from the config in background.
// The caller must wait on the wait group before using the returned container.
// Connection failures are fatal.
func Init(wg *sync.WaitGroup, cfg ConnectionsConfig) *Container {
	cont := NewContainer()

	wg.Go(func() {
		for name, redisCfg := range cfg {
			if !redisCfg.GetEnabled() {
				continue
			}

			if err := cont.Connect(name, redisCfg); err != nil {
				infralog.Fatal("connect to redis", zap.String("connection", name), zap.Error(err))
			}
		}
	})

	return cont
}
