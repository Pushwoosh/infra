package infrarabbit

import (
	"errors"
	"sync"
)

var (
	ErrConfigIsRequired = errors.New("config is required")
	ErrConfigNotFound   = errors.New("config is not found")
)

// Container is a simple container for holding named rabbit connections.
type Container struct {
	mu  *sync.RWMutex
	cfg map[string]*ConnectionConfig
}

func NewContainer() *Container {
	return &Container{
		mu:  &sync.RWMutex{},
		cfg: make(map[string]*ConnectionConfig),
	}
}

// AddConnection adds a named connection to a container.
// It's possible to create consumer or producer on created connection later using CreateProducer ot CreateConsumer.
func (cont *Container) AddConnection(name string, cfg *ConnectionConfig) error {
	cont.mu.Lock()
	cont.cfg[name] = cfg
	cont.mu.Unlock()
	return nil
}

// CreateConsumer creates a new rabbit consumer by a connection name and subscribes it to a given queue
func (cont *Container) CreateConsumer(consumerCfg *ConsumerConfig) (*Consumer, error) {
	cont.mu.Lock()
	defer cont.mu.Unlock()

	if consumerCfg == nil {
		return nil, ErrConfigIsRequired
	}

	cfg, ok := cont.cfg[consumerCfg.ConnectionName]
	if !ok {
		return nil, ErrConfigNotFound
	}

	consumer := &Consumer{
		connCfg: cfg,
		cfg:     consumerCfg,
		ch:      make(chan *Message),
		closed:  make(chan bool),
	}

	go consumer.handle()
	return consumer, nil
}

func (cont *Container) CreateProducer(producerCfg *ProducerConfig) (*Producer, error) {
	cont.mu.Lock()
	defer cont.mu.Unlock()

	if producerCfg == nil {
		return nil, ErrConfigIsRequired
	}

	cfg, ok := cont.cfg[producerCfg.ConnectionName]
	if !ok {
		return nil, ErrConfigNotFound
	}

	p := &Producer{
		connCfg: cfg,
		cfg:     producerCfg,
	}

	return p, p.reconnect()
}
