package infrarabbit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const retryConnectionCountMax = 3

var (
	ErrMessageIsNil             = errors.New("message is nil")
	ErrProducerClosed           = errors.New("producer is closed")
	ErrClosingProducer          = errors.New("error while closing producer")
	ErrUnableToBind             = errors.New("unable to perform binding")
	ErrUnableToCreateChannel    = errors.New("unable to get channel in connection to RabbitMQ")
	ErrUnableToCreateConnection = errors.New("unable to connect to RabbitMQ")
)

type Producer struct {
	connCfg                *ConnectionConfig
	cfg                    *ProducerConfig
	producerAMQPChannel    *amqp.Channel
	producerAMQPConnection *amqp.Connection
	isNeedReconnect        atomic.Bool
	isClosed               atomic.Bool
	isLocked               sync.Mutex
}

type ProducerMessage struct {
	Body       []byte
	Exchange   string
	RoutingKey string
	Priority   uint8
}

func (p *Producer) Produce(pCtx context.Context, msg *ProducerMessage) error {
	p.isLocked.Lock()
	defer p.isLocked.Unlock()

	if msg == nil {
		return ErrMessageIsNil
	}

	if p.isClosed.Load() {
		return ErrProducerClosed
	}

	var err error
	countOfConnectionRetry := 0

	for countOfConnectionRetry < retryConnectionCountMax {
		countOfConnectionRetry++

		if p.producerAMQPChannel != nil && !p.isNeedReconnect.Load() {
			ctx, cancel := context.WithTimeout(pCtx, time.Minute)
			publishErr := p.producerAMQPChannel.PublishWithContext(
				ctx,
				msg.Exchange,
				msg.RoutingKey,
				false,
				false,
				amqp.Publishing{
					Body:      msg.Body,
					Priority:  msg.Priority,
					Timestamp: time.Now(),
				})
			cancel()
			if publishErr == nil {
				return nil
			}

			err = errors.Join(err, publishErr)
		}

		time.Sleep(time.Second)
		if reconnectErr := p.reconnect(p.cfg.Tag); reconnectErr != nil {
			err = errors.Join(err, reconnectErr)
		}
	}

	return err
}

func (p *Producer) handleErrors(ch chan *amqp.Error) {
	go func() {
		for err := range ch {
			if err != nil {
				p.isNeedReconnect.Store(true)
			}
		}
	}()
}

func (p *Producer) reconnect(tag string) error {
	if p.producerAMQPConnection != nil {
		_ = p.producerAMQPConnection.Close()
		p.producerAMQPConnection = nil
	}

	amqpProps := amqp.NewConnectionProperties()
	if tag == "" {
		tag = hostname
	}
	amqpProps.SetClientConnectionName(tag)

	conn, err := amqp.DialConfig(createAMQPURL(p.connCfg), amqp.Config{
		Properties: amqpProps,
	})
	if err != nil {
		return errors.Join(err, ErrUnableToCreateConnection)
	}

	p.handleErrors(conn.NotifyClose(make(chan *amqp.Error)))

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return errors.Join(err, ErrUnableToCreateChannel)
	}

	p.handleErrors(ch.NotifyClose(make(chan *amqp.Error)))

	p.producerAMQPConnection = conn
	p.producerAMQPChannel = ch
	p.isNeedReconnect.Store(false)

	for _, binding := range p.cfg.Bindings {
		if err = bind(ch, binding); err != nil {
			return errors.Join(err, ErrUnableToBind)
		}
	}

	return nil
}

func (p *Producer) Close() error {
	p.isLocked.Lock()
	defer p.isLocked.Unlock()

	if p.isClosed.Swap(true) || p.producerAMQPConnection == nil {
		return nil
	}

	if err := p.producerAMQPConnection.Close(); err != nil {
		return errors.Join(err, ErrClosingProducer)
	}

	return nil
}
