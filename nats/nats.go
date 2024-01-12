package infranats

import (
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

// Container is a simple container for holding named NATS connections.
type Container struct {
	mu *sync.RWMutex

	cfg  map[string]*ConnectionConfig
	pool map[string]*nats.Conn

	wg sync.WaitGroup
}

func NewContainer() *Container {
	return &Container{
		mu:   &sync.RWMutex{},
		cfg:  make(map[string]*ConnectionConfig),
		pool: make(map[string]*nats.Conn),
	}
}

func connErrHandler(conn *nats.Conn, err error) {
	if err == nil {
		return
	}

	infralog.Error("nats connection error", zap.Error(err))
}

func errHandler(conn *nats.Conn, sub *nats.Subscription, err error) {
	if err == nil {
		return
	}

	infralog.Error("nats error", zap.Error(err))
}

// Connect creates a new named NATS connection
func (cont *Container) Connect(name string, cfg *ConnectionConfig) error {
	cont.wg.Add(1)

	conn, err := nats.Connect(cfg.Address, nats.DisconnectErrHandler(connErrHandler), nats.ErrorHandler(errHandler),
		nats.ClosedHandler(func(conn *nats.Conn) {
			cont.wg.Done()
		}))
	if err != nil {
		cont.wg.Done()
		return err
	}

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.pool[name] = conn
	cont.cfg[name] = cfg

	return nil
}

// Get gets connection from a container
func (cont *Container) Get(name string) *nats.Conn {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.pool[name]
}

// Close drains all connection from a container
func (cont *Container) Close() {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	for _, conn := range cont.pool {
		err := conn.Drain()
		if err != nil {
			infralog.Error("can't drain connection", zap.Error(err))
		}
	}

	cont.wg.Wait()
}
