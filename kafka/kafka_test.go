package infrakafka

import (
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func container(t *testing.T) *Container {
	t.Helper()

	cont := NewContainer()
	if err := cont.AddConnection("test", &ConnectionConfig{
		Address: "127.0.0.1:9092", StartOffset: StartOffsetFirst,
	}); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}

	return cont
}

// TestRequiredAcksZeroValue pins the one fact that makes a struct-literal writer
// silently unacknowledged. kafka.NewWriter(WriterConfig) normalises zero to
// RequireAll, which is why reading the library is misleading here.
func TestRequiredAcksZeroValue(t *testing.T) {
	if kafka.RequiredAcks(0) != kafka.RequireNone {
		t.Fatalf("zero value of RequiredAcks is %v, expected RequireNone", kafka.RequiredAcks(0))
	}
}

// TestCreateProducerDefaults characterises CreateProducer rather than endorsing
// it. Every assertion here is a property that costs correctness, and the point of
// the test is that changing any of them changes behaviour for every existing
// caller — so the fix for a caller that needs ordering is
// CreateOrderedProducer, not editing these defaults.
func TestCreateProducerDefaults(t *testing.T) {
	w, err := container(t).CreateProducer("test")
	if err != nil {
		t.Fatalf("CreateProducer: %v", err)
	}

	if w.Balancer != nil {
		t.Errorf("Balancer = %T, expected nil (kafka-go then partitions round-robin)", w.Balancer)
	}

	if w.RequiredAcks != kafka.RequireNone {
		t.Errorf("RequiredAcks = %v, expected RequireNone", w.RequiredAcks)
	}

	if !w.Async {
		t.Error("Async = false, expected true")
	}
}

func TestCreateOrderedProducer(t *testing.T) {
	w, err := container(t).CreateOrderedProducer("test")
	if err != nil {
		t.Fatalf("CreateOrderedProducer: %v", err)
	}

	if _, ok := w.Balancer.(*kafka.Hash); !ok {
		t.Errorf("Balancer = %T, expected *kafka.Hash", w.Balancer)
	}

	if w.RequiredAcks != kafka.RequireAll {
		t.Errorf("RequiredAcks = %v, expected RequireAll", w.RequiredAcks)
	}

	if w.Async {
		t.Error("Async = true, expected false so WriteMessages returns the error")
	}

	if w.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, expected 5", w.MaxAttempts)
	}

	// 1 ms and not 5: a synchronous caller waits out the timeout whenever the
	// batch does not fill, and 5 ms measured 6.7 ms median.
	if w.BatchTimeout != time.Millisecond {
		t.Errorf("BatchTimeout = %v, expected 1ms", w.BatchTimeout)
	}
}

// TestWithCompletion covers the shape a hot path wants: keyed partitioning and
// visible errors without blocking the caller.
func TestWithCompletion(t *testing.T) {
	var seen bool

	w, err := container(t).CreateProducer("test",
		WithHashBalancer(),
		WithRequiredAcks(kafka.RequireAll),
		WithCompletion(func([]kafka.Message, error) { seen = true }))
	if err != nil {
		t.Fatalf("CreateProducer: %v", err)
	}

	if !w.Async {
		t.Error("Async = false, WithCompletion is for the asynchronous shape")
	}

	if w.Completion == nil {
		t.Fatal("Completion not set")
	}

	w.Completion(nil, nil)

	if !seen {
		t.Error("the registered callback was not the one stored")
	}
}

// TestCreateOrderedProducerOverridable checks that the bundled defaults are a
// starting point and not a cage.
func TestCreateOrderedProducerOverridable(t *testing.T) {
	w, err := container(t).CreateOrderedProducer("test",
		WithRequiredAcks(kafka.RequireOne), WithMaxAttempts(3))
	if err != nil {
		t.Fatalf("CreateOrderedProducer: %v", err)
	}

	if w.RequiredAcks != kafka.RequireOne {
		t.Errorf("RequiredAcks = %v, expected the override RequireOne", w.RequiredAcks)
	}

	if w.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, expected the override 3", w.MaxAttempts)
	}

	if _, ok := w.Balancer.(*kafka.Hash); !ok {
		t.Errorf("Balancer = %T, an unrelated override should not have cleared it", w.Balancer)
	}
}

func TestProducerOptions(t *testing.T) {
	w, err := container(t).CreateProducer("test",
		WithHashBalancer(), WithSync(), WithBatch(200, 5*time.Millisecond))
	if err != nil {
		t.Fatalf("CreateProducer: %v", err)
	}

	if _, ok := w.Balancer.(*kafka.Hash); !ok {
		t.Errorf("Balancer = %T, expected *kafka.Hash", w.Balancer)
	}

	if w.Async {
		t.Error("Async = true, expected false")
	}

	if w.BatchSize != 200 || w.BatchTimeout != 5*time.Millisecond {
		t.Errorf("batch = %d/%v, expected 200/5ms", w.BatchSize, w.BatchTimeout)
	}
}

// TestConsumerCommitInterval pins the difference that decides whether a consumer
// may hold a message without finishing it.
func TestConsumerCommitInterval(t *testing.T) {
	cont := container(t)

	def, err := cont.CreateConsumer("test", "grp", []string{"topic"})
	if err != nil {
		t.Fatalf("CreateConsumer: %v", err)
	}

	if got := def.Config().CommitInterval; got != 5*time.Second {
		t.Errorf("default CommitInterval = %v, expected 5s", got)
	}

	manual, err := cont.CreateConsumer("test", "grp", []string{"topic"}, WithManualCommit())
	if err != nil {
		t.Fatalf("CreateConsumer: %v", err)
	}

	if got := manual.Config().CommitInterval; got != 0 {
		t.Errorf("CommitInterval with WithManualCommit = %v, expected 0 (synchronous CommitMessages)", got)
	}
}

func TestUnknownConnection(t *testing.T) {
	if _, err := container(t).CreateOrderedProducer("nope"); err == nil {
		t.Error("expected an error for an unknown connection name")
	}
}
