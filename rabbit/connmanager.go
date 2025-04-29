package infrarabbit

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var hostname = os.Getenv("HOSTNAME")

type connItem struct {
	cfg      *ConnectionConfig
	amqpConn *amqp.Connection
	channels map[*channelItem]bool
	isDead   atomic.Bool
}

func (ci *connItem) MarkAsDead() {
	if ci == nil {
		return
	}

	ci.isDead.Store(true)
}

type channelItem struct {
	cfg                *ConsumerConfig
	messagesInProgress sync.WaitGroup
	amqpChannel        *amqp.Channel
	deliveries         <-chan amqp.Delivery
	isDead             atomic.Bool
}

func (ci *channelItem) InProgressIncrement() {
	ci.messagesInProgress.Add(1)
}

func (ci *channelItem) InProgressDecrement() {
	ci.messagesInProgress.Done()
}

func (ci *channelItem) MarkAsDead() {
	if ci == nil {
		return
	}

	ci.isDead.Store(true)
}

func (ci *channelItem) errCallback(err error) {
	if ci == nil || ci.cfg == nil || ci.cfg.ErrCallback == nil || err == nil {
		return
	}

	ci.cfg.ErrCallback(err)
}

type connManager struct {
	mu    sync.Mutex
	conns map[*connItem]string
}

func newConnManager() *connManager {
	cm := &connManager{
		conns: make(map[*connItem]string),
	}

	go cm.collectMetrics()
	go cm.cleanDeadConnections()

	return cm
}

func (cm *connManager) GetChannel(connCfg *ConnectionConfig, consumerCfg *ConsumerConfig) (*connItem, *channelItem, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	connectionItem, err := cm.createConnection(connCfg, consumerCfg.Tag)
	if err != nil {
		return nil, nil, err
	}

	chItem, err := cm.createChannel(connectionItem, consumerCfg)
	if err != nil {
		return nil, nil, err
	}

	connectionItem.channels[chItem] = true
	return connectionItem, chItem, nil
}

func (cm *connManager) createChannel(connItem *connItem, consumerCfg *ConsumerConfig) (*channelItem, error) {
	channel, err := connItem.amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	prefetchCount := consumerCfg.PrefetchCount
	if prefetchCount < 1 {
		prefetchCount = defaultPrefetchCount
	}

	// very important to set prefetch count,
	// or you may get memory leak!
	if err = channel.Qos(prefetchCount, 0, false); err != nil {
		return nil, err
	}

	queuePriority := consumerCfg.QueuePriority
	args := amqp.Table{}
	if queuePriority > 0 {
		args[PriorityProperty] = int(queuePriority)
	}

	_, err = channel.QueueDeclare(
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

	chItem := &channelItem{
		cfg:         consumerCfg,
		amqpChannel: channel,
	}

	cm.handleChannelErrors(chItem)

	deliveries, err := channel.Consume(
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

func (cm *connManager) createConnection(cfg *ConnectionConfig, tag string) (*connItem, error) {
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

	cItem := &connItem{
		cfg:      cfg,
		amqpConn: conn,
		channels: make(map[*channelItem]bool),
	}

	cm.handleConnErrors(cItem)
	cm.conns[cItem] = amqpURL
	return cItem, nil
}

func (cm *connManager) handleChannelErrors(chItem *channelItem) {
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

func (cm *connManager) handleConnErrors(cItem *connItem) {
	go func() {
		errorsCh := cItem.amqpConn.NotifyClose(make(chan *amqp.Error))
		for connErr := range errorsCh {
			if connErr != nil { // where to put errors?
				cItem.MarkAsDead()
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

			for channel := range conn.channels {
				if channel.isDead.Load() {
					continue
				}

				go collectMetrics(conn, channel)
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

			for channel := range conn.channels {
				if !channel.isDead.Load() && !conn.isDead.Load() {
					continue
				}

				delete(conn.channels, channel)
				go cm.closeDeadChannel(channel)
			}
		}
		cm.mu.Unlock()
	}
}

func (cm *connManager) closeDeadConnection(conn *connItem) {
	_ = conn.amqpConn.Close() // where to put errors?
}

func (cm *connManager) closeDeadChannel(channel *channelItem) {
	channel.messagesInProgress.Wait()
	err := channel.amqpChannel.Close()

	channel.errCallback(err)
}

func collectMetrics(connItem *connItem, chItem *channelItem) {
	defer func() {
		if e := recover(); e != nil {
			chItem.errCallback(fmt.Errorf("%v", e))
			return
		}
	}()

	queue := chItem.cfg.Queue
	metrics := chItem.cfg.Metrics
	host, _ := getHostPort(connItem.cfg.Address)

	if chItem.amqpChannel.IsClosed() || metrics == nil {
		return
	}

	q, err := chItem.amqpChannel.QueueDeclarePassive(
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

	msg, ok, err := chItem.amqpChannel.Get(queue, false)
	if err == nil && ok {
		seconds := time.Since(msg.Timestamp).Seconds()
		_ = msg.Reject(true)
		if seconds >= 0 {
			metrics.QueueDelay(host, queue, int64(seconds))
		}
	}
}
