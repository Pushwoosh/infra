package netretry

import (
	"net"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

const MaxSleep = 5 * time.Second
const MaxRetries = 100

func ExecWithRetry(fn func() error) error {
	sleep := 50 * time.Millisecond
	try := 0
	var lastErr error
	for {
		try++
		err := fn()
		if err == nil {
			return nil
		}

		if !isErrorRetryable(err) {
			return err
		}

		lastErr = err
		if try > MaxRetries {
			return errors.Wrapf(lastErr, "retried %d times", MaxRetries)
		}

		time.Sleep(sleep)
		sleep *= 2
		if sleep > MaxSleep {
			sleep = MaxSleep
		}
	}
}

func isErrorRetryable(err error) bool {
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		if netOpErr.Temporary() {
			return true
		}
	}

	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	return false
}
