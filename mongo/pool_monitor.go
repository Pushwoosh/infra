package mongo

import (
	"math"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/v2/event"
)

var poolMetrics struct {
	Events           *prometheus.CounterVec
	OpenConnections  *prometheus.GaugeVec
	InUseConnections *prometheus.GaugeVec
	PendingCheckouts *prometheus.GaugeVec
	CheckoutDuration *prometheus.HistogramVec
}
var poolMetricsOnce sync.Once

func initPoolMetrics() {
	poolMetricsOnce.Do(func() {
		poolMetrics.Events = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mongo_pool_events_total",
			Help: "The total number of connection pool events by type",
		}, []string{"app", "event"})

		poolMetrics.OpenConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mongo_pool_open_connections",
			Help: "The current number of open connections in the pool",
		}, []string{"app"})

		poolMetrics.InUseConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mongo_pool_in_use_connections",
			Help: "The current number of checked out (in use) connections",
		}, []string{"app"})

		poolMetrics.PendingCheckouts = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mongo_pool_pending_checkouts",
			Help: "The current number of in-flight connection checkout requests",
		}, []string{"app"})

		poolMetrics.CheckoutDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "mongo_pool_checkout_duration",
			Help:    "Time spent waiting to check out a connection from the pool",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, math.Inf(1)},
		}, []string{"app"})

		prometheus.MustRegister(
			poolMetrics.Events,
			poolMetrics.OpenConnections,
			poolMetrics.InUseConnections,
			poolMetrics.PendingCheckouts,
			poolMetrics.CheckoutDuration,
		)
	})
}

// newPoolMonitor builds a driver pool monitor that translates connection pool
// events into Prometheus metrics. The gauges are maintained via inc/dec on the
// paired events (created/closed, checked out/in, checkout started/finished).
func newPoolMonitor(name string) *event.PoolMonitor {
	initPoolMetrics()

	return &event.PoolMonitor{
		Event: func(e *event.PoolEvent) {
			poolMetrics.Events.WithLabelValues(name, e.Type).Inc()

			switch e.Type {
			case event.ConnectionCreated:
				poolMetrics.OpenConnections.WithLabelValues(name).Inc()
			case event.ConnectionClosed:
				poolMetrics.OpenConnections.WithLabelValues(name).Dec()
			case event.ConnectionCheckOutStarted:
				poolMetrics.PendingCheckouts.WithLabelValues(name).Inc()
			case event.ConnectionCheckedOut:
				poolMetrics.PendingCheckouts.WithLabelValues(name).Dec()
				poolMetrics.InUseConnections.WithLabelValues(name).Inc()
				poolMetrics.CheckoutDuration.WithLabelValues(name).Observe(e.Duration.Seconds())
			case event.ConnectionCheckOutFailed:
				poolMetrics.PendingCheckouts.WithLabelValues(name).Dec()
			case event.ConnectionCheckedIn:
				poolMetrics.InUseConnections.WithLabelValues(name).Dec()
			case event.ConnectionPoolCleared:
				// On pool clear the driver closes all idle connections and the
				// in-use ones are dropped on return, so reset the derived gauges
				// to avoid drift.
				poolMetrics.InUseConnections.WithLabelValues(name).Set(0)
				poolMetrics.OpenConnections.WithLabelValues(name).Set(0)
			}
		},
	}
}
