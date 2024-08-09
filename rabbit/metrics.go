package infrarabbit

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	MetricsNamespace = ""

	metricsSourceOnce sync.Once

	consumedMessagesCount  *prometheus.CounterVec
	consumedMessagesStatus *prometheus.CounterVec
	ConsumerQueueLength    *prometheus.GaugeVec
	ConsumerQueueDelay     *prometheus.GaugeVec
)

var initMetrics = func() {
	metricsSourceOnce.Do(func() {
		if MetricsNamespace == "" {
			fmt.Printf("warning: MetricsNamespace should by defined to collect metrics\n")
			return
		}

		consumedMessagesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Name:      "queue_consumed_messages_count_total",
		}, []string{"queue_host", "queue"})
		consumedMessagesStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Name:      "queue_consumed_messages_ack_status_count_total",
		}, []string{"queue_host", "queue", "status"})
		ConsumerQueueLength = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Name:      "queue_length",
		}, []string{"queue_host", "queue"})
		ConsumerQueueDelay = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Name:      "queue_delay_seconds",
		}, []string{"queue_host", "queue"})

		prometheus.MustRegister(
			consumedMessagesCount,
			consumedMessagesStatus,
			ConsumerQueueLength,
			ConsumerQueueDelay)
	})
}
