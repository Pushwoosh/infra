package infrasystem

import (
	"os/exec"
	"os/signal"
	"syscall"
	"testing"

	infraoperator "github.com/pushwoosh/infra/operator"
)

func TestDefaultSignalsKeepChildProcessesWaitable(t *testing.T) {
	defer signal.Reset()

	NewDefaultSignals(&infraoperator.Operator{})

	if signal.Ignored(syscall.SIGCHLD) {
		t.Fatal("SIGCHLD is ignored")
	}

	if err := exec.Command("true").Run(); err != nil {
		t.Fatalf("child process: %v", err)
	}
}
