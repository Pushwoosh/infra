package infrapostgres

import (
	"database/sql"

	"sync"

	"github.com/dlmiddlecote/sqlstats"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Container is a simple container for holding named postgres connections.
// It's a helper for sharing same connection between several components.
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

// Connect creates a new named postgres connection
func (cont *Container) Connect(name string, cfg *ConnectionConfig) error {
	c, err := pgx.ParseConfig(cfg.PGXConnString())
	if err != nil {
		return errors.Wrap(err, "pgx.ParseConfig")
	}

	connStr := stdlib.RegisterConnConfig(c)
	conn, err := sql.Open("pgx", connStr)
	if err != nil {
		return errors.Wrapf(err, "sql.Open")
	}

	err = conn.Ping()
	if err != nil {
		return errors.Wrapf(err, "conn.Ping")
	}

	conn.SetMaxOpenConns(cfg.MaxConnections)
	conn.SetMaxIdleConns(cfg.MaxIdleConnections)
	conn.SetConnMaxIdleTime(cfg.MaxConnectionIdleTime)
	conn.SetConnMaxLifetime(cfg.MaxConnectionLifetime)

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
