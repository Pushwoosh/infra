package infrarabbit

import (
	"sync"
	"sync/atomic"
	"time"
)

var connectionsManager = newConnManager()

type Consumer struct {
	connCfg  *ConnectionConfig
	cfg      *ConsumerConfig
	ch       chan *Message
	mu       sync.Mutex
	closed   chan bool
	isClosed atomic.Bool
}

func (c *Consumer) handle() {
	for !c.isClosed.Load() {
		conn, ch, err := connectionsManager.GetChannel(c.connCfg, c.cfg)
		if err != nil {
			conn.MarkAsDead()
			c.errCallback(err)
			time.Sleep(time.Second)
			continue
		}

		for !c.isClosed.Load() && !ch.isDead.Load() {
			msg, isOpen := <-ch.deliveries
			if !isOpen {
				break
			}

			ch.InProgressIncrement()
			c.ch <- &Message{
				msg: &msg,
				callback: func(err error) {
					ch.InProgressDecrement()
					if err != nil {
						ch.MarkAsDead()
						c.errCallback(err)
					}
				},
			}
		}

		ch.MarkAsDead()
	}

	close(c.ch)
	close(c.closed)
}

func (c *Consumer) errCallback(err error) {
	if c.cfg.ErrCallback == nil || err == nil {
		return
	}

	c.cfg.ErrCallback(err)
}

func (c *Consumer) Consume() chan *Message {
	return c.ch
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed.Swap(true) {
		return nil
	}

	<-c.closed
	return nil
}
