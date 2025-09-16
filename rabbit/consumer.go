package infrarabbit

import (
	"sync/atomic"
	"time"

	infralog "github.com/pushwoosh/infra/log"
	"go.uber.org/zap"
)

var connectionsManager = newConnManager()

type Consumer struct {
	connCfg  *ConnectionConfig
	cfg      *ConsumerConfig
	ch       chan *Message
	closed   chan bool
	isClosed atomic.Bool
}

func (c *Consumer) handle() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for !c.isClosed.Load() {
		ch, err := connectionsManager.GetChannel(c.connCfg, c.cfg)
		if err != nil {
			infralog.Error("unable to get channel", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

	innerLoop:
		for !c.isClosed.Load() && !ch.isDead.Load() {
			select {
			case msg, isOpen := <-ch.deliveries:
				if !isOpen {
					break innerLoop
				}

				ch.InProgressIncrement()
				c.ch <- &Message{
					msg: &msg,
					callback: func(err error) {
						ch.InProgressDecrement()
						if err != nil {
							ch.MarkAsDead()
							infralog.Error("message callback error", zap.Error(err))
						}
					},
				}
			case <-ticker.C:
				// do nothing
			}
		}

		ch.MarkAsDead()
	}

	close(c.ch)
	close(c.closed)
}

func (c *Consumer) Consume() chan *Message {
	return c.ch
}

func (c *Consumer) Close() error {
	if c.isClosed.Swap(true) {
		return nil
	}

	<-c.closed
	return nil
}
