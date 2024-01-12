package infraoperator

import "context"

// Starter interface is used to start background processes
type Starter interface {
	// Start should block until all underlying services are started
	Start(ctx context.Context) error
}

// Stopper interface is used to stop background processed
type Stopper interface {
	// Stop should block until all underlying services are stopped
	Stop(ctx context.Context) error
}

// Checker interface is used for live-checks in background processed
type Checker interface {
	// Check should block until all underlying services reported their status
	Check(ctx context.Context) error
}
