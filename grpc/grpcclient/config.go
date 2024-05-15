package infragrpcclient

import (
	"math"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
)

// ConnectionConfig holds GRPC client configuration
type ConnectionConfig struct {
	// GRPC Service address.
	// See https://github.com/grpc/grpc/blob/master/doc/naming.md for format. TLDR: "host:port" is okay
	Address string `mapstructure:"address"`

	// Keepalive options
	Keepalive *KeepaliveConfig

	// Retry policy config
	Retry *RetryConfig `mapstructure:"retry"`

	// Max outgoing grpc request size in megabytes
	MaxGrpcSendMsgSizeMB int `mapstructure:"max_grpc_send_msg_size_mb"`

	// Max incoming grpc request size in megabytes
	MaxGrpcRecvMsgSizeMB int `mapstructure:"max_grpc_recv_msg_size_mb"`

	// TLSConfig: unused
	TLS *TLSConfig `mapstructure:"tls"`

	// Lazy default false. Setting it to true will give you an opportunity not to connect immediately
	Lazy bool `mapstructure:"lazy"`
}

type TLSConfig struct {
	Enabled bool
}

type ConnectionsConfig map[string]*ConnectionConfig

// KeepaliveConfig holds grpc keepalive balancing options.
// see keepalive.ClientParameters for details.
type KeepaliveConfig struct {
	Time                time.Duration
	Timeout             time.Duration
	PermitWithoutStream bool `mapstructure:"permit_without_stream"`
}

type RetryConfig struct {
	ExponentialBackoff *ExponentialBackoffConfig `mapstructure:"exponential"`
	LinearBackoff      *LinearBackoffConfig      `mapstructure:"linear"`
	Codes              []codes.Code              `mapstructure:"codes"`
}

type ExponentialBackoffConfig struct {
	// Base delay is the length of the first delay in the retry cycle
	BaseDelay time.Duration `mapstructure:"base_delay"`

	// Jitter fraction
	Jitter float64 `mapstructure:"jitter"`

	// MaxAttempts is the cap for retry attempts. Set 0 to disable
	MaxAttempts int `mapstructure:"max_attempts"`
}

type LinearBackoffConfig struct {
	// Delay is the delay length between retries
	Delay time.Duration

	// Jitter fraction
	Jitter float64 `mapstructure:"jitter"`

	// MaxAttempts is the cap for retry attempts. Set 0 to disable
	MaxAttempts int
}

func (c *ConnectionsConfig) Validate() error {
	if c == nil {
		return nil
	}

	for name, conf := range *c {
		if err := conf.Validate(); err != nil {
			return errors.Wrap(err, name)
		}
	}

	return nil
}

func (c *ConnectionConfig) Validate() error {
	if c == nil {
		return errors.New("empty connection config")
	}

	if c.Address == "" {
		return errors.New("Address is mandatory")
	}

	if c.Retry == nil {
		c.Retry = NewDefaultRetryConfig()
	}

	if c.Retry != nil {
		if err := c.Retry.Validate(); err != nil {
			return errors.Wrap(err, "retry")
		}
	}

	return nil
}

func (c *RetryConfig) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.ExponentialBackoff != nil {
		if err := c.ExponentialBackoff.Validate(); err != nil {
			return errors.Wrap(err, "exponential")
		}
	} else if c.LinearBackoff != nil {
		if err := c.LinearBackoff.Validate(); err != nil {
			return errors.Wrap(err, "linear")
		}
	}

	return nil
}

func (c *ExponentialBackoffConfig) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.BaseDelay <= 0 {
		return errors.New("delay should be greater than zero")
	}

	if c.Jitter <= 0 || c.Jitter > 1 {
		return errors.New("jitter must be greater than 0 and less than or equal to 1")
	}

	if c.MaxAttempts < 0 {
		return errors.New("max_attempts should be greater than or equal to 0")
	}

	return nil
}

func (c *LinearBackoffConfig) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Delay <= 0 {
		return errors.New("delay should be greater than zero")
	}

	if c.MaxAttempts < 0 {
		return errors.New("max_attempts should be greater than or equal to 0")
	}

	return nil
}

func NewDefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		LinearBackoff: &LinearBackoffConfig{
			Delay:       time.Millisecond * 50,
			MaxAttempts: math.MaxInt64,
		},
	}
}
