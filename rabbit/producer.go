package infrarabbit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retryConnectionCountMax           = 3
	retryProducerTimeout              = 1 * time.Second
	publishTimeout                    = 30 * time.Second
	intervalToCheckIsNeedReconnect    = 200 * time.Millisecond
	intervalToCheckIsConnectionClosed = 200 * time.Millisecond
)

type Producer struct {
	connCfg                      *ConnectionConfig
	cfg                          *ProducerConfig
	producerAMQPChannel          *amqp.Channel
	producerAMQPConnection       *amqp.Connection
	producerAMQPConnectionErrors chan *amqp.Error
	producerAMQPChannelErrors    chan *amqp.Error
	isNeedReconnect              bool
	isLocked                     sync.Mutex
	isClosed                     bool
}

type ProducerMessage struct {
	Body       []byte
	Exchange   string
	RoutingKey string
	Priority   uint8
}

func (p *Producer) start() error {
	if err := p.reconnect(); err != nil {
		return errors.Wrap(err, "unable to create initial connection to RabbitMQ")
	}

	go func() {
		ticker := time.NewTicker(intervalToCheckIsConnectionClosed)
		defer ticker.Stop()

		for {
			if p.isClosed {
				return
			}
			select {
			case ev, isOpen := <-p.producerAMQPChannelErrors:
				if ev != nil || !isOpen {
					p.isNeedReconnect = true
					time.Sleep(intervalToCheckIsNeedReconnect)
				}
			case ev, isOpen := <-p.producerAMQPConnectionErrors:
				if ev != nil || !isOpen {
					p.isNeedReconnect = true
					time.Sleep(intervalToCheckIsNeedReconnect)
				}
			case <-ticker.C:
				continue
			}
		}
	}()

	return nil
}

func (p *Producer) Produce(pCtx context.Context, msg *ProducerMessage) error {
	p.isLocked.Lock()
	defer p.isLocked.Unlock()

	if msg == nil {
		return errors.New("message is nil")
	}
	if p.isClosed {
		return errors.New("AMQP producer is closed")
	}
	if p.isNeedReconnect {
		if reconnectErr := p.reconnect(); reconnectErr != nil {
			return errors.Wrap(reconnectErr, "isNeedReconnect is true: unable to reconnect in Produce()")
		}
	}

	var err error
	countOfConnectionRetry := 0
	lastErrors := make([]string, 0)

	for countOfConnectionRetry < retryConnectionCountMax {
		countOfConnectionRetry++

		ctx, cancel := context.WithTimeout(pCtx, publishTimeout)
		defer cancel()

		if p.producerAMQPChannel != nil {
			if err = p.producerAMQPChannel.PublishWithContext(
				ctx,
				msg.Exchange,
				msg.RoutingKey,
				false,
				false,
				amqp.Publishing{
					Body:      msg.Body,
					Priority:  msg.Priority,
					Timestamp: time.Now(),
				},
			); err == nil {
				return nil
			} else {
				lastErrors = append(lastErrors, err.Error())
			}
		}

		time.Sleep(retryProducerTimeout)
		if reconnectErr := p.reconnect(); reconnectErr != nil {
			lastErrors = append(lastErrors, fmt.Sprintf("unable to reconnect in Produce(), next try: %d, %s",
				countOfConnectionRetry,
				reconnectErr))
		}
	}

	return errors.Errorf("unable to produce AMQP message: %s", strings.Join(lastErrors, ": "))
}

func (p *Producer) reconnect() error {
	if p.producerAMQPConnection != nil {
		_ = p.producerAMQPConnection.Close()
		p.producerAMQPConnection = nil
	}

	// create all exchanges/bindings/queues if they are not exists
	b, err := NewBinder(p.connCfg)
	if err != nil {
		return errors.Wrap(err, "unable to create binder for creating bindings")
	}
	defer func() {
		_ = b.Close()
	}()
	for _, binding := range p.cfg.Bindings {
		if bindingErr := b.Bind(binding); bindingErr != nil {
			return errors.Wrap(bindingErr, "unable to perform binding")
		}
	}

	// connect to RabbitMQ
	url, err := createAMQPURL(p.connCfg)
	if err != nil {
		return errors.Wrap(err, "unable to create URL for producer")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return errors.Wrap(err, "unable to connect to RabbitMQ")
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return errors.Wrap(err, "unable to get channel in connection to RabbitMQ")
	}

	producerAMQPChannelErrors := make(chan *amqp.Error)
	ch.NotifyClose(producerAMQPChannelErrors)
	p.producerAMQPChannelErrors = producerAMQPChannelErrors
	p.producerAMQPChannel = ch

	producerAMQPConnectionErrors := make(chan *amqp.Error)
	conn.NotifyClose(producerAMQPConnectionErrors)
	p.producerAMQPConnectionErrors = producerAMQPConnectionErrors
	p.producerAMQPConnection = conn

	p.isNeedReconnect = false

	return nil
}

func (p *Producer) Close() error {
	p.isLocked.Lock()
	defer p.isLocked.Unlock()

	if !p.isClosed {
		p.isClosed = true
		if p.producerAMQPConnection != nil {
			if err := p.producerAMQPConnection.Close(); err != nil {
				return errors.Wrap(err, "unable to close AMQP producer connection")
			}
		}
	}

	return nil
}
