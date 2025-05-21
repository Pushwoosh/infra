package infraclickhouse

import (
	"database/sql"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/dlmiddlecote/sqlstats"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Container is a simple container for holding named clickhouse connections.
type Container struct {
	mu         *sync.RWMutex
	cfg        map[string]ConnectionConfig
	conns      map[string]*sql.DB
	collectors map[string]*sqlstats.StatsCollector
}

func NewContainer() *Container {
	return &Container{
		mu:         &sync.RWMutex{},
		cfg:        make(map[string]ConnectionConfig),
		conns:      make(map[string]*sql.DB),
		collectors: make(map[string]*sqlstats.StatsCollector),
	}
}

// Connect creates a new named clickhouse connection
func (cont *Container) Connect(name string, cfg *ConnectionConfig) error {
	dsn := cfg.GetConnectionDSN()
	conn, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return errors.Wrapf(err, "sql.Open")
	}

	err = conn.Ping()
	if err != nil {
		return errors.Wrapf(err, "conn.Ping")
	}

	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 10
	}
	conn.SetMaxOpenConns(cfg.MaxConnections)

	if cfg.MaxIdleConnections <= 0 {
		cfg.MaxIdleConnections = 4
	}
	conn.SetMaxIdleConns(cfg.MaxIdleConnections)

	if cfg.MaxConnectionLifetime <= 0 {
		cfg.MaxConnectionLifetime = 1 * time.Hour
	}
	conn.SetConnMaxLifetime(cfg.MaxConnectionLifetime)

	if cfg.MaxConnectionIdleTime <= 0 {
		cfg.MaxConnectionIdleTime = 10 * time.Second
	}
	conn.SetConnMaxIdleTime(cfg.MaxConnectionIdleTime)

	if collector := cont.GetCollector(name); collector != nil {
		prometheus.Unregister(collector)
	}
	collector := sqlstats.NewStatsCollector(name, conn)
	prometheus.MustRegister(collector)

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.conns[name] = conn
	cont.cfg[name] = *cfg
	cont.collectors[name] = collector

	return nil
}

// Get gets connection from a container
func (cont *Container) Get(name string) *sql.DB {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.conns[name]
}

// GetCollector gets metrics collector from a container
func (cont *Container) GetCollector(name string) *sqlstats.StatsCollector {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.collectors[name]
}
