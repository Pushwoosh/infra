package infrarabbit

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type connManager struct {
	mu          sync.Mutex
	connections map[*amqp.Connection]string
}

func newConnManager() *connManager {
	return &connManager{
		connections: make(map[*amqp.Connection]string),
	}
}

func (cp *connManager) Get(cfg *ConnectionConfig, tag string) (*amqp.Connection, bool, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	amqpURL, err := createAMQPURL(cfg)
	if err != nil {
		return nil, false, errors.Wrap(err, "unable to build AMQP URL")
	}
	for conn, url := range cp.connections {
		if url == amqpURL {
			return conn, false, nil
		}
	}

	amqpProps := amqp.NewConnectionProperties()
	if tag != "" {
		amqpProps.SetClientConnectionName(tag)
	}

	conn, err := amqp.DialConfig(amqpURL, amqp.Config{
		Properties: amqpProps,
	})
	if err != nil {
		return nil, false, errors.Wrap(err, "unable to connect rabbitmq")
	}
	cp.connections[conn] = amqpURL

	return conn, true, nil
}

func (cp *connManager) CreateConsumerChannel(
	conn *amqp.Connection,
	tag string,
	queue string,
	prefetchCount int,
) (*amqp.Channel, <-chan amqp.Delivery, error) {
	channel, err := conn.Channel()
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create rabbitmq channel")
	}

	if prefetchCount < 1 {
		prefetchCount = defaultPrefetchCount
	}
	// very important to set prefetch count
	// or you may get memory leak!
	if err = channel.Qos(prefetchCount, 0, false); err != nil {
		return nil, nil, errors.Wrap(err, "unable to set QoS")
	}

	_, err = channel.QueueDeclare(
		queue, // name of the queue
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to declare queue")
	}

	deliveries, err := channel.Consume(
		queue, // queue name
		tag,   // consumerTag,
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to get deliveries")
	}

	return channel, deliveries, nil
}

func (cp *connManager) CloseConnection(conn *amqp.Connection) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, connectionExists := cp.connections[conn]; connectionExists {
		delete(cp.connections, conn)
		go func() {
			// close amqp connections in separate goroutine because
			// connection.Close() may block forever
			if err := conn.Close(); err != nil {
				infralog.Error("unable to close connection", zap.Error(err))
			}
		}()
	}
}

func (cp *connManager) CloseConsumerChannel(channel *amqp.Channel) {
	go func() {
		// close amqp channel in separate goroutine because
		// channel.Close() may block forever
		if err := channel.Close(); err != nil {
			infralog.Error("unable to close channel", zap.Error(err))
		}
	}()
}
