package infrarabbit

import (
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Message struct {
	msg      *amqp.Delivery
	callback func(error)
	once     atomic.Bool
}

func (m *Message) Ack() error {
	if m.once.Swap(true) {
		return nil
	}

	err := m.msg.Ack(false)
	m.callback(err)

	return err
}

func (m *Message) Nack() error {
	if m.once.Swap(true) {
		return nil
	}

	err := m.msg.Nack(false, true)
	m.callback(err)

	return err
}

func (m *Message) IsRedelivered() bool {
	return m.msg.Redelivered
}

func (m *Message) Body() []byte {
	return m.msg.Body
}
