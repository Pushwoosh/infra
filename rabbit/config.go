package infrarabbit

import (
	"time"

	"errors"
)

const (
	defaultPrefetchCount = 1
	defaultVHost         = "/"
	defaultUser          = "guest"
	defaultPassword      = "guest"

	PriorityProperty = "x-max-priority"
)

var (
	ErrAddressIsRequired = errors.New("address is mandatory")
)

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
			return errors.Join(err, errors.New(name))
		}
	}

	return nil
}

func (c *ConnectionConfig) Validate() error {
	if c == nil {
		return ErrConfigIsRequired
	}

	if c.Address == "" {
		return ErrAddressIsRequired
	}

	return nil
}
