package infragrpcclient

import (
	"context"
	"fmt"
	"net"
	"slices"
	"testing"

	"github.com/pkg/errors"
	"github.com/pushwoosh/infra/grpc/grpcclient/testpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

// Test_ClientInterceptor tests ability to create client interceptor.
// It creates a grpc server which test metadata. Metadata is added by client interceptor.
func Test_ClientInterceptor(t *testing.T) {
	const port = 4444
	// create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(localAddress, port))
	if err != nil {
		t.Error(err)
	}

	// run example grpc server
	grpcServer := grpc.NewServer()
	testpb.RegisterHelloServiceServer(grpcServer, &testInterceptorServer{})
	go func() {
		srvErr := grpcServer.Serve(listener)
		if srvErr != nil {
			t.Error(srvErr)
		}
	}()

	defer func() { grpcServer.Stop() }()

	cont := NewContainer()
	err = cont.Connect("test", &ConnectionConfig{
		Address: fmt.Sprintf(localAddress, port),
	}, grpc.WithChainUnaryInterceptor(testInterceptor))
	if err != nil {
		t.Error(err)
	}

	conn := cont.Get("test")
	cl := testpb.NewHelloServiceClient(conn)
	resp, err := cl.Hello(context.Background(), &testpb.HelloRequest{Message: "hello world"})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Reply)
}

type testInterceptorServer struct {
	testpb.UnimplementedHelloServiceServer
}

func (s *testInterceptorServer) Hello(ctx context.Context, in *testpb.HelloRequest) (*testpb.HelloResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("metadata not found")
	}

	if len(md.Get("key")) == 0 {
		return nil, errors.New("metadata not found")
	}

	sl := []byte(in.Message)
	slices.Reverse(sl)
	return &testpb.HelloResponse{Reply: string(sl)}, nil
}

func testInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("key", "value"))
	return invoker(ctx, method, req, reply, cc, opts...)
}
