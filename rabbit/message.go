package infrarabbit

import (
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	statusAck       = "ack"
	statusAckError  = "ack-error"
	statusNack      = "nack"
	statusNackError = "nack-error"
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
		consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusAckError).Inc()
		return err
	}

	m.callback(nil)
	consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusAck).Inc()

	return nil
}

func (m *Message) Nack() error {
	if m.once.Swap(true) {
		return nil
	}

	if err := m.msg.Nack(false, true); err != nil {
		m.callback(err)
		consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusNackError).Inc()
		return err
	}

	m.callback(nil)
	consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusNack).Inc()

	return nil
}

func (m *Message) IsRedelivered() bool {
	return m.msg.Redelivered
}

func (m *Message) Body() []byte {
	return m.msg.Body
}
