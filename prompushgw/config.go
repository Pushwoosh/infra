package infraprompushgw

import (
	"github.com/pkg/errors"
)

type Config struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Address == "" {
		return errors.New("address is required")
	}

	return nil
}
