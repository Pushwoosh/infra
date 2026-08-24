package infrakafka

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// ProducerOption adjusts a writer returned by CreateProducer or
// CreateOrderedProducer.
type ProducerOption func(*kafka.Writer)

// WithHashBalancer makes the writer place every message carrying the same key in
// the same partition.
//
// Without it kafka-go falls back to round-robin and the message key has no
// effect at all, which is easy to miss: the code looks like it partitions by key
// and does not. Measured on kafka-go v0.4.47 — a thousand messages sharing one
// key spread evenly over all eight partitions of a topic.
func WithHashBalancer() ProducerOption {
	return func(w *kafka.Writer) { w.Balancer = &kafka.Hash{} }
}

// WithBalancer sets an explicit balancer, for the cases where Hash is not what
// you want.
func WithBalancer(b kafka.Balancer) ProducerOption {
	return func(w *kafka.Writer) { w.Balancer = b }
}

// WithRequiredAcks sets how many replicas must acknowledge a write.
//
// The zero value of kafka.RequiredAcks is RequireNone, so a writer built as a
// struct literal without this field asks the broker for no acknowledgement at
// all. Note that the deprecated kafka.NewWriter(WriterConfig) path normalises
// zero to RequireAll, which makes the difference easy to overlook when reading
// the library.
func WithRequiredAcks(acks kafka.RequiredAcks) ProducerOption {
	return func(w *kafka.Writer) { w.RequiredAcks = acks }
}

// WithSync makes WriteMessages block until the write is acknowledged and return
// the error if it was not.
//
// With Async left at true — which is what CreateProducer gives you —
// WriteMessages returns before the broker has seen the message and, in the
// library's own words, "errors are ignored". A caller that answers its own client
// "accepted" after such a write is guessing.
func WithSync() ProducerOption {
	return func(w *kafka.Writer) { w.Async = false }
}

// WithMaxAttempts caps the retries of a single write. CreateProducer defaults to
// 1000, which is closer to hanging than to retrying.
func WithMaxAttempts(n int) ProducerOption {
	return func(w *kafka.Writer) { w.MaxAttempts = n }
}

// WithBatch tunes how much a synchronous writer accumulates before sending,
// which is how the latency of WithSync is paid back.
func WithBatch(size int, timeout time.Duration) ProducerOption {
	return func(w *kafka.Writer) {
		w.BatchSize = size
		w.BatchTimeout = timeout
	}
}

// WithCompletion registers a callback invoked once a write resolves, with the
// error if it failed.
//
// This is what makes an asynchronous writer accountable. Async means "do not
// block the caller", not "do not confirm": the batch still goes out with whatever
// RequiredAcks says, and the outcome arrives here instead of at the call site. A
// producer on a hot path can therefore keep microsecond latency and still count,
// log and alert on what it lost — measured 1 microsecond median against 1.6
// milliseconds for the synchronous equivalent.
//
// What it cannot do is tell the caller its write failed, because the caller was
// answered long before. Use it where "accepted" was already the honest answer,
// and WithSync where it was not.
func WithCompletion(fn func(msgs []kafka.Message, err error)) ProducerOption {
	return func(w *kafka.Writer) { w.Completion = fn }
}

// ConsumerOption adjusts a reader returned by CreateConsumer.
type ConsumerOption func(*kafka.ReaderConfig)

// WithManualCommit turns off periodic offset commits, so offsets advance only
// when the caller calls CommitMessages, and that call becomes synchronous.
//
// Required by any consumer that may hold a message without finishing it — a
// reorder buffer, a retry queue, a batch that is still filling. CreateConsumer
// otherwise commits every 5 seconds, and ReadMessage commits as it reads, so a
// message still in flight is already committed and is gone after a rebalance.
// Use FetchMessage rather than ReadMessage together with this option.
func WithManualCommit() ConsumerOption {
	return func(c *kafka.ReaderConfig) { c.CommitInterval = 0 }
}

// WithCommitInterval overrides the commit interval. Zero means synchronous
// commits; see WithManualCommit.
func WithCommitInterval(d time.Duration) ConsumerOption {
	return func(c *kafka.ReaderConfig) { c.CommitInterval = d }
}

// WithConsumerMaxAttempts caps the retries of a single fetch. CreateConsumer
// defaults to 1000.
func WithConsumerMaxAttempts(n int) ConsumerOption {
	return func(c *kafka.ReaderConfig) { c.MaxAttempts = n }
}

// WithQueueCapacity sets the reader's internal prefetch queue.
func WithQueueCapacity(n int) ConsumerOption {
	return func(c *kafka.ReaderConfig) { c.QueueCapacity = n }
}
