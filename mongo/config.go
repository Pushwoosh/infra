package mongo

import (
	"time"

	"github.com/pkg/errors"
)

type ConnectionsConfig map[string]*ConnectionConfig

type ConnectionConfig struct {
	// mongodb://user:password@host:port/dbname?options
	// https://www.mongodb.com/docs/manual/reference/connection-string/
	URI string `mapstructure:"uri"`

	// Query log config
	QueryLog *QueryLoggingConfig `mapstructure:"query_log"`
}

type QueryLoggingConfig struct {
	// Whether to log all queries
	All bool `mapstructure:"all"`

	// Whether to log slow queries
	Slow bool `mapstructure:"slow"`

	// Queries that were executed longer than that time will appear in slow log
	SlowThreshold time.Duration `mapstructure:"slow_threshold"`
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

	if err := c.QueryLog.Validate(); err != nil {
		return errors.Wrap(err, "query_log")
	}

	if c.URI == "" {
		return errors.New("uri is empty")
	}

	return nil
}

func (c *QueryLoggingConfig) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Slow && c.SlowThreshold == 0 {
		return errors.New("slow threshold must be greater than zero")
	}

	return nil
}
