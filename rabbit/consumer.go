package infrarabbit

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	defaultPrefetchCount = 1
	defaultVHost         = "/"
	defaultUser          = "guest"
	defaultPassword      = "guest"

	connCloseChanSize             = 8096
	metricsIntervalCheck          = 15 * time.Second
	heartbeatIntervalCheck        = time.Second
	heartbeatReconnectionInterval = 5 * time.Minute
)

var connectionsManager = newConnManager()

type Consumer struct {
	connCfg  *ConnectionConfig
	cfg      *ConsumerConfig
	ch       chan *Message
	mu       sync.Mutex
	closed   chan bool
	isClosed bool
}

func (c *Consumer) start() {
	cfg := c.cfg
	host, _ := getHostPort(c.connCfg.Address)

	metricsTicker := time.NewTicker(metricsIntervalCheck)
	defer metricsTicker.Stop()
	heartbeatTicker := time.NewTicker(heartbeatIntervalCheck)
	defer heartbeatTicker.Stop()

reconnectLoop:
	for !c.isClosed {
		conn, isNewConn, err := connectionsManager.Get(c.connCfg, cfg.Tag)
		if err != nil {
			time.Sleep(time.Second) // time to wait to not make infinite "for" loop
			continue
		}

		channel, deliveries, err := connectionsManager.CreateConsumerChannel(conn, cfg.Tag, cfg.Queue, cfg.PrefetchCount)
		if err != nil {
			connectionsManager.CloseConnection(conn)
			time.Sleep(time.Second) // time to wait to not make infinite "for" loop
			continue
		}

		channelClose := channel.NotifyClose(make(chan *amqp.Error, connCloseChanSize))
		var connClose chan *amqp.Error
		if isNewConn {
			connClose = conn.NotifyClose(make(chan *amqp.Error, connCloseChanSize))
		}

		lastTimeConnectionUsed := time.Now()
		isNeedRecreateChannel := false

		var errCallback = func(err error) {
			isNeedRecreateChannel = true
		}

		for !c.isClosed {
			select {
			case closeErr, isOpen := <-connClose:
				if closeErr != nil || !isOpen {
					go readAllErrors(connClose)
					connectionsManager.CloseConnection(conn)
					continue reconnectLoop
				}
			case closeErr, isOpen := <-channelClose:
				if closeErr != nil || !isOpen {
					go readAllErrors(channelClose)
					connectionsManager.CloseConsumerChannel(channel)
					continue reconnectLoop
				}
			case <-heartbeatTicker.C:
				if time.Since(lastTimeConnectionUsed) > heartbeatReconnectionInterval || isNeedRecreateChannel {
					connectionsManager.CloseConsumerChannel(channel)
					continue reconnectLoop
				}
			case <-metricsTicker.C:
				go collectMetrics(channel, host, cfg.Queue)
			case msg, isOpen := <-deliveries:
				if !isOpen {
					connectionsManager.CloseConsumerChannel(channel)
					continue reconnectLoop
				}
				lastTimeConnectionUsed = time.Now()
				consumedMessagesCount.WithLabelValues(host, cfg.Queue).Inc()
				c.ch <- &Message{
					msg:         &msg,
					host:        host,
					queue:       cfg.Queue,
					errCallback: errCallback,
				}
			}
		}
	}
	close(c.ch)
	close(c.closed)
}

func (c *Consumer) Consume() chan *Message {
	return c.ch
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return nil
	}

	c.isClosed = true
	<-c.closed
	return nil
}

func collectMetrics(channel *amqp.Channel, host string, queue string) {
	defer func() {
		if e := recover(); e != nil {
			infralog.Error(
				"unable to collect rabbit metrics",
				zap.Error(errors.Errorf("%v", e)))
		}
	}()

	if channel != nil {
		q, err := channel.QueueDeclarePassive(
			queue,
			false, // durable
			false, // delete when unused
			false, // exclusive
			false, // noWait
			nil,   // arguments
		)
		if err == nil {
			queueLength := float64(q.Messages)
			ConsumerQueueLength.WithLabelValues(host, queue).Set(queueLength)
			if q.Messages == 0 {
				ConsumerQueueDelay.WithLabelValues(host, queue).Set(0)
				return
			}
		}

		msg, ok, err := channel.Get(queue, false)
		if err == nil && ok {
			seconds := time.Since(msg.Timestamp).Seconds()
			_ = msg.Reject(true)
			if seconds >= 0 {
				ConsumerQueueDelay.WithLabelValues(host, queue).Set(seconds)
			}
		}
	}
}

func readAllErrors(ch chan *amqp.Error) {
	for range ch {
		// need to read all errors to avoid deadlocks
		// https://github.com/rabbitmq/amqp091-go/issues/18
	}
}
