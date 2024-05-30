package infragrpcclient

import (
	"context"
	"sync"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	defaultRetryDelay       = time.Second
	defaultRetryMaxAttempts = 10
	defaultRetryJitter      = 0.3
	connectionTimeout       = 5 * time.Second
)

var ErrFailedToConnect = errors.New("failed to bring connection to ready state")

// nolint:gochecknoinits
func init() {
	sync.OnceFunc(func() {
		grpc_prometheus.EnableClientHandlingTimeHistogram()
	})()
}

// Container is a simple container for holding named GRPC connections.
type Container struct {
	mu   *sync.RWMutex
	cfg  map[string]ConnectionConfig
	pool map[string]grpc.ClientConnInterface
}

func NewContainer() *Container {
	return &Container{
		mu:   &sync.RWMutex{},
		cfg:  make(map[string]ConnectionConfig),
		pool: make(map[string]grpc.ClientConnInterface),
	}
}

// Connect creates a new named GRPC connection
func (cont *Container) Connect(name string, cfg *ConnectionConfig) error {
	unaryInterceptors := []grpc.UnaryClientInterceptor{
		grpc_prometheus.UnaryClientInterceptor,
	}

	if cfg.Retry == nil {
		cfg.Retry = &RetryConfig{
			LinearBackoff: &LinearBackoffConfig{
				Delay:       defaultRetryDelay,
				MaxAttempts: defaultRetryMaxAttempts,
				Jitter:      defaultRetryJitter,
			},
		}
	}

	codes := grpc_retry.DefaultRetriableCodes
	if cfg.Retry.Codes != nil {
		codes = cfg.Retry.Codes
	}

	if cfg.Retry.ExponentialBackoff != nil {
		backoff := cfg.Retry.ExponentialBackoff
		if backoff.Jitter <= 0 || backoff.Jitter > 1 {
			backoff.Jitter = defaultRetryJitter
		}
		unaryInterceptors = append(unaryInterceptors,
			grpc_retry.UnaryClientInterceptor(
				grpc_retry.WithBackoff(grpc_retry.BackoffExponentialWithJitter(backoff.BaseDelay, backoff.Jitter)),
				grpc_retry.WithMax(uint(backoff.MaxAttempts)),
				grpc_retry.WithCodes(codes...),
			),
		)
	} else if cfg.Retry.LinearBackoff != nil {
		backoff := cfg.Retry.LinearBackoff
		if backoff.Jitter <= 0 || backoff.Jitter > 1 {
			backoff.Jitter = defaultRetryJitter
		}
		unaryInterceptors = append(unaryInterceptors,
			grpc_retry.UnaryClientInterceptor(
				grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(backoff.Delay, backoff.Jitter)),
				grpc_retry.WithMax(uint(backoff.MaxAttempts)),
				grpc_retry.WithCodes(codes...),
			),
		)
	} else if cfg.Retry.Codes != nil {
		unaryInterceptors = append(unaryInterceptors,
			grpc_retry.UnaryClientInterceptor(
				grpc_retry.WithCodes(codes...),
			),
		)
	} else {
		panic("invalid retryer config")
	}

	options := []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(unaryInterceptors...),
	}

	if cfg.TLS == nil || !cfg.TLS.Enabled {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		panic("TLS is not supported yet")
	}

	// setup keepalive options. see keepalive.ClientParameters for details
	if cfg.Keepalive != nil {
		options = append(options, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cfg.Keepalive.Time,
			Timeout:             cfg.Keepalive.Timeout,
			PermitWithoutStream: cfg.Keepalive.PermitWithoutStream,
		}))
	}

	// setup load balancing options
	options = append(options, grpc.WithDefaultServiceConfig(
		`{"loadBalancingPolicy":"`+roundrobin.Name+`"}`),
	)

	if cfg.MaxGrpcSendMsgSizeMB > 0 {
		options = append(options, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(cfg.MaxGrpcSendMsgSizeMB*1024*1024)))
	}

	if cfg.MaxGrpcRecvMsgSizeMB > 0 {
		options = append(options, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.MaxGrpcRecvMsgSizeMB*1024*1024)))
	}

	conn, err := grpc.NewClient(cfg.Address, options...)
	if err != nil {
		return errors.Wrapf(err, "can't create grpc connection to \"%s\"", cfg.Address)
	}

	// try to connect immediately if lazy connection is disabled
	if !cfg.Lazy {
		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		defer cancel()

		conn.Connect()

		conn.WaitForStateChange(ctx, connectivity.Idle) // do not check result, we are not interested in that change
		stateChanged := conn.WaitForStateChange(ctx, connectivity.Connecting)
		if !stateChanged || conn.GetState() != connectivity.Ready {
			return errors.Wrap(ErrFailedToConnect, name)
		}
	}

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.pool[name] = conn
	cont.cfg[name] = *cfg

	return nil
}

// Get gets connection from a container
func (cont *Container) Get(name string) grpc.ClientConnInterface {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.pool[name]
}
