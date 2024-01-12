package netretry_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/pushwoosh/infra/netretry"
)

func Test_example_netretry(t *testing.T) {
	t.Skip()
	startServer(t)

	try := 0
	err := netretry.ExecWithRetry(func() error {
		try++
		fmt.Printf("try: %d\n", try)
		conn, err := net.Dial("tcp", ":4242")
		if err != nil {
			return err
		}

		// write two bytes, server reads one byte and closes connection
		if _, err = conn.Write([]byte("ab")); err != nil {
			return err
		}

		// if the server has data in the buffer and connection
		// is closed, the read will return connection reset by peer
		if _, err = conn.Read(make([]byte, 1)); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
}

// nolint:unused
func startServer(t *testing.T) {
	listener, err := net.Listen("tcp", ":4242")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		defer func() { _ = listener.Close() }()
		for {
			conn, err := listener.Accept()
			if err != nil {
				t.Log(err)
				return
			}

			// read one byte and close connection
			if _, err := conn.Read(make([]byte, 1)); err != nil {
				t.Log(err)
				return
			}

			_ = conn.Close()
		}
	}()
}
