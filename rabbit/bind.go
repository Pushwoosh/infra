package infrarabbit

import (
	"sync"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Kind string

const KindDirect Kind = "direct"
const KindFanOut Kind = "fanout"
const KindTopic Kind = "topic"
const KindHeaders Kind = "headers"

type binder struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	isLocked sync.Mutex
	isClosed bool
}

type BindConfig struct {
	Exchange   string // required
	RoutingKey string // required
	Queue      string // required

	// optional
	ExchangeKind       Kind
	ExchangeDurable    bool
	ExchangeAutoDelete bool
	ExchangeInternal   bool
	ExchangeNoWait     bool
	ExchangeArgs       map[string]interface{}
	QueueDurable       bool
	QueueAutoDelete    bool
	QueueExclusive     bool
	QueueNoWait        bool
	QueueArgs          map[string]interface{}
	BindNoWait         bool
	BindArgs           map[string]interface{}
}

func NewBinder(config *ConnectionConfig) (*binder, error) {
	url, err := createAMQPURL(config)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create URL for binder")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to RabbitMQ")
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(err, "unable to get channel from RabbitMQ")
	}

	return &binder{
		conn:    conn,
		channel: ch,
	}, nil
}

func (b *binder) Bind(config *BindConfig) error {
	b.isLocked.Lock()
	defer b.isLocked.Unlock()

	if b.isClosed {
		return errors.New("binder is already closed")
	}

	exchangeKind := config.ExchangeKind
	if exchangeKind == "" {
		exchangeKind = KindDirect
	}
	if err := b.channel.ExchangeDeclare(
		config.Exchange,
		string(exchangeKind),
		config.ExchangeDurable,
		config.ExchangeAutoDelete,
		config.ExchangeInternal,
		config.ExchangeNoWait,
		config.ExchangeArgs); err != nil {
		return errors.Wrap(err, "unable to declare exchange")
	}

	if _, err := b.channel.QueueDeclare(
		config.Queue,
		config.QueueDurable,
		config.QueueAutoDelete,
		config.QueueExclusive,
		config.QueueNoWait,
		config.QueueArgs); err != nil {
		return errors.Wrap(err, "unable to declare queue")
	}

	if err := b.channel.QueueBind(
		config.Queue,
		config.RoutingKey,
		config.Exchange,
		config.BindNoWait,
		config.BindArgs); err != nil {
		return errors.Wrap(err, "unable to bind queue to exchange")
	}

	return nil
}

func (b *binder) Close() error {
	b.isLocked.Lock()
	defer b.isLocked.Unlock()

	if !b.isClosed {
		b.isClosed = true
		if b.conn != nil {
			if err := b.conn.Close(); err != nil {
				return errors.Wrap(err, "unable to close connection for binder")
			}
		}
	}
	return nil
}
