package infragrpcserver

import "github.com/pkg/errors"

const GrpcCapacityUnlimited = 0
const DefaultGrpcListenAddress = ":9091"
const DefaultGrpcCapacity = GrpcCapacityUnlimited

type GrpcConfig struct {
	Listen            string         `mapstructure:"listen"`
	Capacity          int            `mapstructure:"capacity"`
	ReflectionEnabled bool           `mapstructure:"reflection_enabled"`
	GrpcWeb           *GrpcWebConfig `mapstructure:"grpc_web"`
}

type GrpcWebConfig struct {
	Listen string `mapstructure:"listen"`
}

func (c *GrpcWebConfig) Validate() error {
	if c == nil {
		return nil
	}

	if c.Listen == "" {
		return errors.New("listen address is mandatory")
	}

	return nil
}

func DefaultGrpcConfig() *GrpcConfig {
	return &GrpcConfig{
		Listen:            DefaultGrpcListenAddress,
		Capacity:          DefaultGrpcCapacity,
		ReflectionEnabled: true,
	}
}

func (c *GrpcConfig) Validate() error {
	if c.Capacity < 0 {
		return errors.New("capacity must bre greater or equal to zero")
	}

	if c.Listen == "" {
		return errors.New("listen address is mandatory")
	}

	if err := c.GrpcWeb.Validate(); err != nil {
		return err
	}

	return nil
}
