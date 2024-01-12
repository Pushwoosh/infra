package infralog

import (
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

// Config is a logger configuration structure.
type Config struct {
	Environment string
	Level       string
}

const (
	levelDebug   = "debug"
	levelInfo    = "info"
	levelWarn    = "warn"
	levelError   = "error"
	LevelDefault = levelInfo

	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
	EnvironmentDefault     = EnvironmentProduction
)

var logLevels = map[string]zapcore.Level{
	levelDebug: zapcore.DebugLevel,
	levelInfo:  zapcore.InfoLevel,
	levelWarn:  zapcore.WarnLevel,
	levelError: zapcore.ErrorLevel,
}

func DefaultConfig() *Config {
	return &Config{
		Environment: EnvironmentDefault,
		Level:       LevelDefault,
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("empty config")
	}

	if c.Level != "" {
		if _, ok := logLevels[c.Level]; !ok {
			return errors.New("invalid log level \"%s\". expected \"debug\" or \"info\" or \"warn\" or \"error\"" + c.Level)
		}
	}

	if c.Environment == "" {
		c.Environment = EnvironmentDefault
	}

	if c.Environment != EnvironmentDevelopment && c.Environment != EnvironmentProduction {
		return errors.Errorf("invalid environment \"%s\". expected \"developemnt\" or \"production\"", c.Environment)
	}

	return nil
}

func (c *Config) GetLogLevel() zapcore.Level {
	return logLevels[c.Level]
}
