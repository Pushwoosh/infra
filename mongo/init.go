package mongo

import (
	"sync"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

// Init creates a container and connects every connection from the config in background.
// appName is reported to the mongo server as the client application name.
// The caller must wait on the wait group before using the returned container.
// Connection failures are fatal.
func Init(wg *sync.WaitGroup, appName string, cfg ConnectionsConfig) *Container {
	cont := NewContainer()

	wg.Go(func() {
		for name, mongoCfg := range cfg {
			if err := cont.Connect(appName, name, mongoCfg); err != nil {
				infralog.Fatal("connect to mongo", zap.String("connection", name), zap.Error(err))
			}
		}
	})

	return cont
}
