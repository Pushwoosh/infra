package infrahttp

import (
	"github.com/pkg/errors"
)

type Config struct {
	Listen string `mapstructure:"listen"`
}

func DefaultConfig() *Config {
	return &Config{
		Listen: ":8081",
	}
}

func (c *Config) Validate() error {
	if c.Listen == "" {
		return errors.New("listen address is mandatory")
	}

	return nil
}
