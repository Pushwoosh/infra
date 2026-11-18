package infraredis

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	infralog "github.com/pushwoosh/infra/log"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// connectRetryMaxInterval caps the delay between connection attempts.
const connectRetryMaxInterval = 5 * time.Second

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

	if err := ping(name, conn, cfg.GetConnectRetryTimeout()); err != nil {
		_ = conn.Close()
		return errors.Wrap(err, "cannot create redis connection")
	}

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.pool[name] = conn
	cont.cfg[name] = *cfg

	return nil
}

// ping checks that redis is reachable. Failed attempts are retried with
// exponential backoff until retryTimeout elapses; the last error is returned then.
func ping(name string, conn redis.UniversalClient, retryTimeout time.Duration) error {
	attempt := func() error {
		return conn.Ping(context.Background()).Err()
	}

	if retryTimeout <= 0 {
		return attempt()
	}

	policy := backoff.NewExponentialBackOff(
		backoff.WithMaxInterval(connectRetryMaxInterval),
		backoff.WithMaxElapsedTime(retryTimeout),
	)

	return backoff.RetryNotify(attempt, policy, func(err error, next time.Duration) {
		infralog.Warn("redis is unavailable, retrying",
			zap.String("connection", name),
			zap.Duration("retry_in", next),
			zap.Error(err),
		)
	})
}

// Get gets connection from a container
func (cont *Container) Get(name string) redis.UniversalClient {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.pool[name]
}
