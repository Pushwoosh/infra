package infrarabbit

import (
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Message struct {
	msg      *amqp.Delivery
	host     string
	queue    string
	callback func(error)
	once     atomic.Bool
}

func (m *Message) Ack() error {
	if m.once.Swap(true) {
		return nil
	}

	if err := m.msg.Ack(false); err != nil {
		m.callback(err)
		return err
	}

	m.callback(nil)
	return nil
}

func (m *Message) Nack() error {
	if m.once.Swap(true) {
		return nil
	}

	if err := m.msg.Nack(false, true); err != nil {
		m.callback(err)
		return err
	}

	m.callback(nil)
	return nil
}

func (m *Message) IsRedelivered() bool {
	return m.msg.Redelivered
}

func (m *Message) Body() []byte {
	return m.msg.Body
}
