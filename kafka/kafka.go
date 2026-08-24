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

// CreateProducer creates a new kafka producer by a connection name.
//
// The writer it returns is deliberately fire-and-forget, and that is worth
// stating plainly because the defaults are not obvious from the call site:
//
//   - no Balancer, so kafka-go partitions round-robin and the message key is
//     ignored — a keyed write does NOT keep a key's messages together;
//   - no RequiredAcks, whose zero value is RequireNone, so the broker confirms
//     nothing;
//   - Async: true, which per the library's documentation means write errors are
//     discarded.
//
// That is the right shape for a fan-out stream where a lost message costs
// nothing. It is the wrong shape whenever the order or the durability of a write
// matters — use CreateOrderedProducer, or pass the options in options.go.
func (cont *Container) CreateProducer(connectionName string, opts ...ProducerOption) (*kafka.Writer, error) {
	cont.mu.Lock()
	defer cont.mu.Unlock()

	kafkaConfig, ok := cont.cfg[connectionName]
	if !ok {
		return nil, errors.Errorf("invalid connection name: \"%s\"", connectionName)
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(kafkaConfig.Address),
		AllowAutoTopicCreation: true,
		Async:                  true,
		MaxAttempts:            1000,
		Logger:                 kafka.LoggerFunc(getLogFunc()),
		ErrorLogger:            kafka.LoggerFunc(getLogErrorFunc()),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// CreateOrderedProducer creates a producer that partitions by message key and
// confirms every write: Hash balancer, RequireAll, synchronous, and a retry count
// that gives up rather than hangs.
//
// Use it where the caller must learn that its write failed. Measured against a
// local broker at 32 concurrent writers: 1.6 ms median, 2.1 ms at the 99th
// percentile, 15 000 messages a second through one writer. That is affordable for
// a few thousand writes a second and wasteful for tens of thousands.
//
// The batch timeout is deliberately 1 ms rather than 5. A synchronous caller
// waits out the timeout whenever the batch does not fill, and at 5 ms that wait
// measured 6.7 ms median — enough to dominate a request that otherwise takes 3.
//
// For a hot path where "accepted" is an honest answer, prefer CreateProducer with
// WithHashBalancer and WithCompletion: same partitioning, errors still visible,
// and no wait at all. Options here are applied last, so any of this can be
// overridden.
func (cont *Container) CreateOrderedProducer(connectionName string, opts ...ProducerOption) (*kafka.Writer, error) {
	defaults := []ProducerOption{
		WithHashBalancer(),
		WithRequiredAcks(kafka.RequireAll),
		WithSync(),
		WithMaxAttempts(5),
		WithBatch(200, time.Millisecond),
	}

	return cont.CreateProducer(connectionName, append(defaults, opts...)...)
}

// CreateConsumer creates a new kafka consumer by a connection name and subscribes it to a given topic.
//
// Offsets are committed every 5 seconds, and Reader.ReadMessage commits as it
// reads. That suits a consumer that finishes with a message before asking for the
// next one. It does not suit one that may hold a message unfinished — a reorder
// buffer, a retry queue, a batch still filling — because such a message is
// committed while still in flight and will not be redelivered after a rebalance.
// For that, pass WithManualCommit and read with FetchMessage.
func (cont *Container) CreateConsumer(
	connectionName string,
	consumerGroup string,
	topics []string,
	opts ...ConsumerOption,
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

	cfg := kafka.ReaderConfig{
		Brokers:        []string{kafkaConfig.Address},
		GroupID:        consumerGroup,
		GroupTopics:    topics,
		QueueCapacity:  0,
		CommitInterval: 5 * time.Second,
		StartOffset:    offset,
		Logger:         kafka.LoggerFunc(getLogFunc()),
		ErrorLogger:    kafka.LoggerFunc(getLogErrorFunc()),
		MaxAttempts:    1000,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return kafka.NewReader(cfg), nil
}
