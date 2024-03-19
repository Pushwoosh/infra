package infrakafka

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Container is a simple container for holding named kafka connections.
type Container struct {
	mu *sync.RWMutex

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

// Exists checks if a connection with the given name exists in the container.
func (cont *Container) Exists(name string) bool {
	_, ok := cont.cfg[name]
	return ok
}

func getLogFunc() func(msg string, a ...interface{}) {
	return func(msg string, a ...interface{}) {
		infralog.Debug("kafka-go", zap.String("value", fmt.Sprintf(msg, a...)))
	}
}

func getLogErrorFunc() func(msg string, a ...interface{}) {
	return func(msg string, a ...interface{}) {
		infralog.Error("kafka-go", zap.Error(errors.Errorf(msg, a...)))
	}
}

// CreateProducer creates a new kafka producer by a connection name
func (cont *Container) CreateProducer(connectionName string) (*kafka.Writer, error) {
	cont.mu.Lock()
	defer cont.mu.Unlock()

	kafkaConfig, ok := cont.cfg[connectionName]
	if !ok {
		return nil, errors.Errorf("invalid connection name: \"%s\"", connectionName)
	}

	return &kafka.Writer{
		Addr:                   kafka.TCP(kafkaConfig.Address),
		AllowAutoTopicCreation: true,
		Async:                  true,
		MaxAttempts:            1000,
		Logger:                 kafka.LoggerFunc(getLogFunc()),
		ErrorLogger:            kafka.LoggerFunc(getLogErrorFunc()),
	}, nil
}

// CreateConsumer creates a new kafka consumer by a connection name and subscribes it to a given topic
func (cont *Container) CreateConsumer(
	connectionName string,
	consumerGroup string,
	topics []string,
) (*kafka.Reader, error) {
	cont.mu.Lock()
	defer cont.mu.Unlock()

	kafkaConfig, ok := cont.cfg[connectionName]
	if !ok {
		return nil, errors.Errorf("invalid connection name: \"%s\"", connectionName)
	}

	var offset int64
	if kafkaConfig.StartOffset == StartOffsetFirst {
		offset = kafka.FirstOffset
	} else {
		offset = kafka.LastOffset
	}

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaConfig.Address},
		GroupID:        consumerGroup,
		GroupTopics:    topics,
		QueueCapacity:  0,
		CommitInterval: 5 * time.Second,
		StartOffset:    offset,
		Logger:         kafka.LoggerFunc(getLogFunc()),
		ErrorLogger:    kafka.LoggerFunc(getLogErrorFunc()),
		MaxAttempts:    1000,
	}), nil
}
