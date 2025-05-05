package infrarabbit

import (
	"errors"
	"sync"
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrBinderIsClosed              = errors.New("binder is closed")
	ErrUnableToCloseConnection     = errors.New("unable to close connection")
	ErrUnableToDeclareExchange     = errors.New("unable to declare exchange")
	ErrUnableToDeclareQueue        = errors.New("unable to declare queue")
	ErrUnableToBindQueueToExchange = errors.New("unable to bind queue to exchange")
)

type Kind string

const KindDirect Kind = "direct"
const KindFanOut Kind = "fanout"
const KindTopic Kind = "topic"
const KindHeaders Kind = "headers"

type Binder struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	isLocked sync.Mutex
	isClosed atomic.Bool
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

func NewBinder(config *ConnectionConfig) (*Binder, error) {
	conn, err := amqp.Dial(createAMQPURL(config))
	if err != nil {
		return nil, errors.Join(err, ErrUnableToCreateConnection)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, errors.Join(err, ErrUnableToCreateChannel)
	}

	return &Binder{
		conn:    conn,
		channel: ch,
	}, nil
}

func (b *Binder) Bind(config *BindConfig) error {
	b.isLocked.Lock()
	defer b.isLocked.Unlock()

	if b.isClosed.Load() {
		return ErrBinderIsClosed
	}

	return bind(b.channel, config)
}

func (b *Binder) Close() error {
	b.isLocked.Lock()
	defer b.isLocked.Unlock()

	if b.isClosed.Swap(true) || b.conn == nil {
		return nil
	}

	if err := b.conn.Close(); err != nil {
		return errors.Join(err, ErrUnableToCloseConnection)
	}

	return nil
}

func bind(channel *amqp.Channel, config *BindConfig) error {
	exchangeKind := config.ExchangeKind
	if exchangeKind == "" {
		exchangeKind = KindDirect
	}

	if err := channel.ExchangeDeclare(
		config.Exchange,
		string(exchangeKind),
		config.ExchangeDurable,
		config.ExchangeAutoDelete,
		config.ExchangeInternal,
		config.ExchangeNoWait,
		config.ExchangeArgs,
	); err != nil {
		return errors.Join(err, ErrUnableToDeclareExchange)
	}

	if _, err := channel.QueueDeclare(
		config.Queue,
		config.QueueDurable,
		config.QueueAutoDelete,
		config.QueueExclusive,
		config.QueueNoWait,
		config.QueueArgs,
	); err != nil {
		return errors.Join(err, ErrUnableToDeclareQueue)
	}

	if err := channel.QueueBind(
		config.Queue,
		config.RoutingKey,
		config.Exchange,
		config.BindNoWait,
		config.BindArgs); err != nil {
		return errors.Join(err, ErrUnableToBindQueueToExchange)
	}

	return nil
}
