package infraoperator

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

type Operator struct {
	mu sync.Mutex

	stoppers collection
	checkers collection
}

// AddService starts the service and puts it under operator's control.
// returns service's start error
func (o *Operator) AddService(ctx context.Context, service interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if service, ok := service.(Starter); ok {
		if err := service.Start(ctx); err != nil {
			return errors.Wrap(err, "service start error")
		}
	}

	if service, ok := service.(Stopper); ok {
		o.stoppers = o.stoppers.add(service)
	}

	if service, ok := service.(Checker); ok {
		o.checkers = o.checkers.add(service)
	}

	return nil
}

// RemoveService stops service and removes it service from operator's control.
// returns service's stop error
func (o *Operator) RemoveService(ctx context.Context, service interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if service, ok := service.(Checker); ok {
		o.checkers = o.checkers.remove(service)
	}

	if service, ok := service.(Stopper); ok {
		o.stoppers = o.stoppers.remove(service)

		if err := service.Stop(ctx); err != nil {
			return errors.Wrap(err, "service stop error")
		}
	}

	return nil
}

// StopAll removes all checks and stops all services.
func (o *Operator) StopAll(ctx context.Context) map[interface{}]error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.checkers = nil

	// stop all services in reverse order
	ret := make(map[interface{}]error)
	for i := len(o.stoppers) - 1; i >= 0; i-- {
		stopper, _ := o.stoppers[i].(Stopper)
		err := stopper.Stop(ctx)
		if err != nil {
			ret[stopper] = err
		}
	}

	return ret
}

// Check does a live-check on all services.
// Returns map with
//
//	key = service reference
//	value = service stop errors
func (o *Operator) Check(ctx context.Context) map[interface{}]error {
	o.mu.Lock()
	defer o.mu.Unlock()

	type checkResult struct {
		service interface{}
		err     error
	}

	checkResults := make(chan checkResult)

	for _, checker := range o.checkers {
		go func(checker Checker) {
			checkResults <- checkResult{checker, checker.Check(ctx)}
		}(checker.(Checker))
	}

	// collect all check errors and create return structure
	ret := make(map[interface{}]error)
	for range o.checkers {
		result := <-checkResults
		if result.err != nil {
			ret[result.service] = result.err
		}
	}

	return ret
}
