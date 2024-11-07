package infralog

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var setup bool

func Setup(cfg *Config) *zap.Logger {
	// allow to call setup only once, because logger is a kind of singleton
	if setup {
		panic("infralog: Setup() already called")
	}
	setup = true

	// build logger
	var loggerConfig zap.Config
	if cfg.Environment == EnvironmentDevelopment {
		loggerConfig = zap.NewDevelopmentConfig()
	} else {
		loggerConfig = zap.NewProductionConfig()
		loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	loggerConfig.Level.SetLevel(cfg.GetLogLevel())
	loggerConfig.DisableStacktrace = cfg.DisableStacktrace

	l, err := loggerConfig.Build()
	if err != nil {
		panic(errors.Wrap(err, "Unable to create Logger"))
	}

	// setup default logging handler that simply passes all logs to the zap logger
	RegisterLogHandler(func(entry *LogEntry) {
		switch entry.Level {
		case zapcore.DebugLevel:
			l.Debug(entry.Message, entry.Fields...)
		case zapcore.InfoLevel:
			l.Info(entry.Message, entry.Fields...)
		case zapcore.WarnLevel:
			l.Warn(entry.Message, entry.Fields...)
		case zapcore.ErrorLevel:
			l.Error(entry.Message, entry.Fields...)
		case zapcore.FatalLevel:
			l.Fatal(entry.Message, entry.Fields...)
		}
	})

	return l
}
