package infraredis

import (
	"time"

	"github.com/pkg/errors"
)

type ConnectionsConfig map[string]*ConnectionConfig

type ConnectionConfig struct {
	// Whether to use cluster client or single node client
	ClusterMode bool `mapstructure:"cluster_mode"`

	// Database Address.
	// if ClusterMode == false then "host:port" is expected
	// if ClusterMode == true  then "host:port[,host:port]" is expected
	Address string `mapstructure:"address"`

	// Read and write timeouts. Default values using getters: -1 (no timeout)
	ReadTimeout  *time.Duration `mapstructure:"read_timeout"`
	WriteTimeout *time.Duration `mapstructure:"write_timeout"`
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

func (c *ConnectionConfig) GetReadTimeout() time.Duration {
	if c.ReadTimeout == nil {
		return -1
	}

	return *c.ReadTimeout
}

func (c *ConnectionConfig) GetWriteTimeout() time.Duration {
	if c.WriteTimeout == nil {
		return -1
	}

	return *c.WriteTimeout
}

func (c *ConnectionConfig) Validate() error {
	if c == nil {
		return errors.New("empty connection config")
	}

	if c.Address == "" {
		return errors.New("address is mandatory")
	}

	return nil
}
