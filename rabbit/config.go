package infrarabbit

import (
	"time"

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

type ConsumerMetrics struct {
	CheckInterval time.Duration                         // optional
	QueueLength   func(host, queue string, value int64) // optional
	QueueDelay    func(host, queue string, value int64) // optional
}

type ConsumerConfig struct {
	ConnectionName string
	Queue          string
	QueuePriority  uint8            // optional
	PrefetchCount  int              // optional
	Tag            string           // optional
	Metrics        *ConsumerMetrics // optional
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
