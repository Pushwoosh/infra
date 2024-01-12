package infrapostgres

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type ConnectionsConfig map[string]*ConnectionConfig

type ConnectionConfig struct {
	// Database address. "host:port"
	Address string `mapstructure:"address"`

	// Database credentials
	Credentials Credentials `mapstructure:"credentials"`

	// Maximum number of physical connections
	MaxConnections int `mapstructure:"max_connections"`

	// Maximum number of idle connections
	MaxIdleConnections int `mapstructure:"max_idle_connections"`

	// Maximum connection lifetime. Connections that active more than that period will be closed
	MaxConnectionLifetime time.Duration `mapstructure:"max_connection_lifetime"`

	// Connection idle time. Connections that idle more than that period will be closed
	MaxConnectionIdleTime time.Duration `mapstructure:"max_connection_idle_time"`

	// Whether to use simple protocol.
	// Extended protocol uses prepared queries.
	// prepared queries are not compatible with PGBouncer in any modes other than session.
	PreferSimpleProtocol bool `mapstructure:"prefer_simple_protocol"`
}

type Credentials struct {
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func (c *ConnectionConfig) PGXConnString() string {
	opts := []string{
		fmt.Sprintf("prefer_simple_protocol=%t", c.PreferSimpleProtocol),
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s?%s",
		c.Credentials.Username,
		c.Credentials.Password,
		c.Address,
		c.Credentials.Database,
		strings.Join(opts, "&"))
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
		return errors.New("empty config")
	}

	if c.Address == "" {
		return errors.New("address is mandatory")
	}

	if err := c.Credentials.Validate(); err != nil {
		return errors.Wrap(err, "credentials")
	}

	return nil
}

func (c *Credentials) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Database == "" {
		return errors.New("database name is mandatory")
	}

	if c.Username == "" {
		return errors.New("username is mandatory")
	}

	return nil
}
