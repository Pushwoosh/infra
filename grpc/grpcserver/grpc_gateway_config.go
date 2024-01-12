package infragrpcserver

import "github.com/pkg/errors"

const (
	DefaultGrpcGatewayListenAddress        = ":8081"
	DefaultGrpcGatewayForwardToAddress     = "127.0.0.1:9091"
	DefaultGrpcGatewayMaxCallRecvMsgSizeMB = 0
	DefaultGrpcGatewayMaxCallSendMsgSizeMB = 0
)

type GrpcGatewayConfig struct {
	Listen               string `mapstructure:"listen"`
	ForwardTo            string `mapstructure:"forward_to"`
	MaxCallRecvMsgSizeMB int    `mapstructure:"max_call_recv_msg_size_mb"`
	MaxCallSendMsgSizeMB int    `mapstructure:"max_call_send_msg_size_mb"`
}

func DefaultGrpcGatewayConfig() *GrpcGatewayConfig {
	return &GrpcGatewayConfig{
		Listen:    DefaultGrpcGatewayListenAddress,
		ForwardTo: DefaultGrpcGatewayForwardToAddress,
	}
}

func (c *GrpcGatewayConfig) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Listen == "" {
		return errors.New("\"listen\" is mandatory")
	}

	if c.ForwardTo == "" {
		return errors.New("\"forward_to\" is mandatory")
	}

	return nil
}
