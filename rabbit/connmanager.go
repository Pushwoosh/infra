package infrarabbit

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	infralog "github.com/pushwoosh/infra/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

var hostname = os.Getenv("HOSTNAME")

type connection struct {
	cfg      *ConnectionConfig
	amqpConn *amqp.Connection
	channels map[*channel]bool
	isDead   atomic.Bool
}

func (ci *connection) MarkAsDead() {
	if ci == nil {
		return
	}

	ci.isDead.Store(true)
}

type channel struct {
	cfg                *ConsumerConfig
	messagesInProgress sync.WaitGroup
	amqpChannel        *amqp.Channel
	deliveries         <-chan amqp.Delivery
	isDead             atomic.Bool
}

func (ch *channel) InProgressIncrement() {
	ch.messagesInProgress.Add(1)
}

func (ch *channel) InProgressDecrement() {
	ch.messagesInProgress.Done()
}

// MarkAsDead marks the channel as dead. ci can be nil.
func (ch *channel) MarkAsDead() {
	if ch == nil {
		return
	}

	ch.isDead.Store(true)
}

func (ch *channel) errCallback(err error) {
	if ch == nil || ch.cfg == nil || ch.cfg.ErrCallback == nil || err == nil {
		return
	}

	ch.cfg.ErrCallback(err)
}

func (ch *channel) collectMetrics(host string) {
	defer func() {
		if e := recover(); e != nil {
			ch.errCallback(fmt.Errorf("%v", e))
			return
		}
	}()

	queue := ch.cfg.Queue
	metrics := ch.cfg.Metrics

	if ch.amqpChannel.IsClosed() || metrics == nil {
		return
	}

	q, err := ch.amqpChannel.QueueDeclarePassive(
		queue,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return
	}

	if metrics.QueueLength != nil {
		metrics.QueueLength(host, queue, int64(q.Messages))
	}

	if metrics.QueueDelay == nil {
		return
	}

	if q.Messages == 0 {
		metrics.QueueDelay(host, queue, 0)
		return
	}

	msg, ok, err := ch.amqpChannel.Get(queue, false)
	if err == nil && ok {
		seconds := time.Since(msg.Timestamp).Seconds()
		_ = msg.Reject(true)
		if seconds >= 0 {
			metrics.QueueDelay(host, queue, int64(seconds))
		}
	}
}

type connManager struct {
	mu    sync.Mutex
	conns map[*connection]string
}

func newConnManager() *connManager {
	cm := &connManager{
		conns: make(map[*connection]string),
	}

	go cm.collectMetrics()
	go cm.cleanDeadConnections()

	return cm
}

func (cm *connManager) GetChannel(connCfg *ConnectionConfig, consumerCfg *ConsumerConfig) (*connection, *channel, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, err := cm.createConnection(connCfg, consumerCfg.Tag)
	if err != nil {
		return nil, nil, err
	}

	ch, err := cm.createChannel(conn, consumerCfg)
	if err != nil {
		return nil, nil, err
	}

	conn.channels[ch] = true
	return conn, ch, nil
}

func (cm *connManager) createChannel(connItem *connection, consumerCfg *ConsumerConfig) (*channel, error) {
	ch, err := connItem.amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	prefetchCount := consumerCfg.PrefetchCount
	if prefetchCount < 1 {
		prefetchCount = defaultPrefetchCount
	}

	// very important to set prefetch count,
	// or you may get memory leak!
	if err = ch.Qos(prefetchCount, 0, false); err != nil {
		return nil, err
	}

	queuePriority := consumerCfg.QueuePriority
	args := amqp.Table{}
	if queuePriority > 0 {
		args[PriorityProperty] = int(queuePriority)
	}

	_, err = ch.QueueDeclare(
		consumerCfg.Queue, // name of the queue
		false,             // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // noWait
		args,              // arguments
	)
	if err != nil {
		return nil, err
	}

	chItem := &channel{
		cfg:         consumerCfg,
		amqpChannel: ch,
	}

	cm.handleChannelErrors(chItem)

	deliveries, err := ch.Consume(
		consumerCfg.Queue, // queue name
		consumerCfg.Tag,   // consumerTag,
		false,             // autoAck
		false,             // exclusive
		false,             // noLocal
		false,             // noWait
		nil,               // arguments
	)
	if err != nil {
		return nil, err
	}

	chItem.deliveries = deliveries
	connItem.channels[chItem] = true
	return chItem, nil
}

func (cm *connManager) createConnection(cfg *ConnectionConfig, tag string) (*connection, error) {
	amqpURL := createAMQPURL(cfg)
	for c, url := range cm.conns {
		if url == amqpURL && !c.isDead.Load() {
			return c, nil
		}
	}

	amqpProps := amqp.NewConnectionProperties()
	if tag == "" {
		tag = hostname
	}
	amqpProps.SetClientConnectionName(tag)

	conn, err := amqp.DialConfig(amqpURL, amqp.Config{
		Properties: amqpProps,
	})
	if err != nil {
		return nil, err
	}

	cItem := &connection{
		cfg:      cfg,
		amqpConn: conn,
		channels: make(map[*channel]bool),
	}

	cm.handleConnErrors(cItem)
	cm.conns[cItem] = amqpURL
	return cItem, nil
}

func (cm *connManager) handleChannelErrors(chItem *channel) {
	go func() {
		errorsCh := chItem.amqpChannel.NotifyClose(make(chan *amqp.Error))
		for err := range errorsCh {
			if err != nil {
				chItem.MarkAsDead()
				chItem.errCallback(err)
			}
		}
	}()
}

func (cm *connManager) handleConnErrors(cItem *connection) {
	go func() {
		errorsCh := cItem.amqpConn.NotifyClose(make(chan *amqp.Error))
		for connErr := range errorsCh {
			if connErr != nil { // where to put errors?
				cItem.MarkAsDead()
				infralog.Error("error on rabbitmq connection", zap.Error(connErr))
			}
		}
	}()
}

func (cm *connManager) collectMetrics() {
	ticker := time.NewTicker(time.Minute / 2)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.Lock()
		for conn := range cm.conns {
			if conn.isDead.Load() {
				continue
			}

			host, _ := getHostPort(conn.cfg.Address)
			for ch := range conn.channels {
				if ch.isDead.Load() {
					continue
				}

				go ch.collectMetrics(host)
			}
		}
		cm.mu.Unlock()
	}
}

func (cm *connManager) cleanDeadConnections() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.Lock()
		for conn := range cm.conns {
			if conn.isDead.Load() && len(conn.channels) == 0 {
				delete(cm.conns, conn)
				go cm.closeDeadConnection(conn)
				continue
			}

			for ch := range conn.channels {
				if !ch.isDead.Load() && !conn.isDead.Load() {
					continue
				}

				delete(conn.channels, ch)
				go cm.closeDeadChannel(ch)
			}
		}
		cm.mu.Unlock()
	}
}

func (cm *connManager) closeDeadConnection(conn *connection) {
	_ = conn.amqpConn.Close() // where to put errors?
}

func (cm *connManager) closeDeadChannel(channel *channel) {
	channel.messagesInProgress.Wait()
	err := channel.amqpChannel.Close()

	channel.errCallback(err)
}
