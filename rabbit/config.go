package infrarabbit

import (
	"github.com/pkg/errors"
)

const PriorityProperty = "x-max-priority"

type ConnectionsConfig map[string]*ConnectionConfig

type ConnectionConfig struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Vhost    string `mapstructure:"vhost"`
}

type ConsumerConfig struct {
	ConnectionName string
	Queue          string
	QueuePriority  uint8
	PrefetchCount  int
	Tag            string
}

type ProducerConfig struct {
	ConnectionName string
	Bindings       []*BindConfig
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

	return nil
}
