package infrainfoserver

import (
	"time"

	"github.com/pkg/errors"
)

const (
	DefaultInfoServerListenAddress = ":8080"
	DefaultInfoServerChecksTimeout = time.Second * 5
)

// Config is an info server configuration structure
type Config struct {
	// Address the server will be bound to
	Listen string `mapstructure:"listen"`

	// Timeout for health check call
	ChecksTimeout time.Duration `mapstructure:"checks_timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		Listen:        DefaultInfoServerListenAddress,
		ChecksTimeout: DefaultInfoServerChecksTimeout,
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Listen == "" {
		return errors.New("empty listen address")
	}

	return nil
}
