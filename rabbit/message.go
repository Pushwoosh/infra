package infrarabbit

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	statusAck       = "ack"
	statusAckError  = "ack-error"
	statusNack      = "nack"
	statusNackError = "nack-error"
)

type Message struct {
	msg         *amqp.Delivery
	host        string
	queue       string
	errCallback func(error)
}

func (m *Message) Ack() error {
	if err := m.msg.Ack(false); err != nil {
		m.errCallback(err)
		consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusAckError).Inc()
		return err
	}
	consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusAck).Inc()

	return nil
}

func (m *Message) Nack() error {
	if err := m.msg.Nack(false, true); err != nil {
		m.errCallback(err)
		consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusNackError).Inc()
		return err
	}
	consumedMessagesStatus.WithLabelValues(m.host, m.queue, statusNack).Inc()

	return nil
}

func (m *Message) IsRedelivered() bool {
	return m.msg.Redelivered
}

func (m *Message) Body() []byte {
	return m.msg.Body
}
