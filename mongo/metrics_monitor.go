package mongo

import (
	"context"
	"math"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/v2/event"
)

var metrics struct {
	StartedQueryCounter    *prometheus.CounterVec
	FinishedQueryCounter   *prometheus.CounterVec
	QueryDurationHistogram *prometheus.HistogramVec
}
var metricsSourceOnce sync.Once

func initMetrics() {
	metricsSourceOnce.Do(func() {
		metrics.StartedQueryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mongo_started_query_counter",
			Help: "The total number of started queries",
		}, []string{"app", "database", "collection", "command"})

		metrics.FinishedQueryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mongo_finished_query_counter",
			Help: "The total number of started queries",
		}, []string{"app", "database", "collection", "command", "status"})

		metrics.QueryDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "mongo_query_duration",
			Help:    "The mongo query duration",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 50, math.Inf(1)},
		}, []string{"app", "database", "collection", "command", "status"})

		prometheus.MustRegister(
			metrics.StartedQueryCounter,
			metrics.FinishedQueryCounter,
			metrics.QueryDurationHistogram,
		)
	})
}

func initMetricsMonitor(b *monitorBuilder, name string) {
	initMetrics()

	b.Add(
		func(ctx context.Context, startedEvent *event.CommandStartedEvent) {
			collection := determineCollection(startedEvent)
			metrics.StartedQueryCounter.WithLabelValues(name, startedEvent.DatabaseName, collection, startedEvent.CommandName).Inc()
		},
		func(ctx context.Context, startedEvent *event.CommandStartedEvent, succeededEvent *event.CommandSucceededEvent) {
			collection := determineCollection(startedEvent)
			metrics.FinishedQueryCounter.WithLabelValues(name, startedEvent.DatabaseName, collection, succeededEvent.CommandName, "success").Inc()
			metrics.QueryDurationHistogram.WithLabelValues(name, startedEvent.DatabaseName, collection, succeededEvent.CommandName, "success").Observe(succeededEvent.Duration.Seconds())
		},
		func(ctx context.Context, startedEvent *event.CommandStartedEvent, failedEvent *event.CommandFailedEvent) {
			collection := determineCollection(startedEvent)
			metrics.FinishedQueryCounter.WithLabelValues(name, startedEvent.DatabaseName, collection, failedEvent.CommandName, "error").Inc()
			metrics.QueryDurationHistogram.WithLabelValues(name, startedEvent.DatabaseName, collection, failedEvent.CommandName, "error").Observe(failedEvent.Duration.Seconds())
		},
	)
}
