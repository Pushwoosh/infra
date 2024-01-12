package infragrpcclient

import (
	"fmt"
	"net"
	"testing"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const localAddress = "127.0.0.1:%d"

func Test_ClientConnectionError(t *testing.T) {
	const port = 4141
	cont := NewContainer()
	err := cont.Connect("test", &ConnectionConfig{
		Address: fmt.Sprintf(localAddress, port),
	})

	if !errors.Is(err, ErrFailedToConnect) {
		t.Errorf("expected failed connection")
	}
}

func Test_ClientConnectionTimeout(t *testing.T) {
	const port = 4242

	// create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(localAddress, port))
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = listener.Close() }()

	// do not create any grpc server on a listener.
	// that will hold all connections unaccepted and hang the client for an infinite time

	// try to connect to created server
	cont := NewContainer()
	err = cont.Connect("test", &ConnectionConfig{
		Address: fmt.Sprintf(localAddress, port),
	})

	// expect error
	if !errors.Is(err, ErrFailedToConnect) {
		t.Errorf("expected failed connection")
	}
}

func Test_ConnectionSuccess(t *testing.T) {
	const port = 4343
	// create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(localAddress, port))
	if err != nil {
		t.Error(err)
	}

	// run an empty grpc server on that listener
	srv := grpc.NewServer()
	go func() {
		srvErr := srv.Serve(listener)
		if srvErr != nil {
			t.Error(srvErr)
		}
	}()
	defer func() { srv.Stop() }()

	// try to connect to created server
	cont := NewContainer()
	err = cont.Connect("test", &ConnectionConfig{
		Address: fmt.Sprintf(localAddress, port),
	})

	// expect successful connection
	if err != nil {
		t.Error(err)
	}

	srv.Stop()
	_ = listener.Close()
}
