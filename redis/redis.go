package infraredis

import (
	"context"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// Container is a simple container for holding named redis connections.
type Container struct {
	mu   *sync.RWMutex
	cfg  map[string]ConnectionConfig
	pool map[string]redis.UniversalClient
}

func NewContainer() *Container {
	return &Container{
		mu:   &sync.RWMutex{},
		cfg:  make(map[string]ConnectionConfig),
		pool: make(map[string]redis.UniversalClient),
	}
}

// Connect creates a new named redis connection
func (cont *Container) Connect(name string, cfg *ConnectionConfig) error {
	var (
		opts redis.UniversalOptions
		conn redis.UniversalClient
	)

	opts.ReadTimeout = cfg.GetReadTimeout()
	opts.WriteTimeout = cfg.GetWriteTimeout()

	// cannot use redis.NewUniversalClient here because it determines whether
	// client should be clustered or not by checking connection nodes count.
	// We use dedicated ClusterMode flag instead.
	if cfg.ClusterMode {
		opts.Addrs = strings.Split(cfg.Address, ",")
		conn = redis.NewClusterClient(opts.Cluster())
	} else {
		opts.Addrs = []string{cfg.Address}
		conn = redis.NewClient(opts.Simple())
	}

	if err := conn.Ping(context.Background()).Err(); err != nil {
		return errors.Wrap(err, "cannot create redis connection")
	}

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.pool[name] = conn
	cont.cfg[name] = *cfg

	return nil
}

// Get gets connection from a container
func (cont *Container) Get(name string) redis.UniversalClient {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.pool[name]
}
