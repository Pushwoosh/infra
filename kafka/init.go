package infrakafka

import (
	"sync"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

// Init creates a container and adds every connection from the config in background.
// The caller must wait on the wait group before using the returned container.
// Connection failures are fatal.
func Init(wg *sync.WaitGroup, cfg ConnectionsConfig) *Container {
	cont := NewContainer()

	wg.Go(func() {
		for name, kafkaCfg := range cfg {
			if err := cont.AddConnection(name, kafkaCfg); err != nil {
				infralog.Fatal("connect to kafka", zap.String("connection", name), zap.Error(err))
			}
		}
	})

	return cont
}
