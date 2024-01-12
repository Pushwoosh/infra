package infrakafka

import (
	"github.com/pkg/errors"
)

type ConnectionsConfig map[string]*ConnectionConfig

type StartOffset string

const (
	StartOffsetFirst StartOffset = "first"
	StartOffsetLast  StartOffset = "last"
)

type ConnectionConfig struct {
	// Broker Address. Comma-separated list of "host:port" expected
	Address     string      `mapstructure:"address"`
	StartOffset StartOffset `mapstructure:"start_offset"`
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
		return errors.New("address is mandatory")
	}

	if len(c.StartOffset) == 0 {
		c.StartOffset = StartOffsetFirst
	}

	if c.StartOffset != StartOffsetFirst && c.StartOffset != StartOffsetLast {
		return errors.New("start_offset must be either 'first' or 'last'")
	}

	return nil
}
