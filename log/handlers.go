package infralog

import (
	"fmt"
	"math"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// SentryErrorLogHandler sends the first error from entry fields to Sentry
func SentryErrorLogHandler() LogHandler {
	return func(entry *LogEntry) {
		if zapcore.WarnLevel.Enabled(entry.Level) {
			err := entry.Error()
			if err != nil {
				sentry.PushScope()
				sentry.ConfigureScope(func(scope *sentry.Scope) {
					for i := range entry.Fields {
						f := &entry.Fields[i]
						if f.Type == zapcore.ErrorType {
							continue
						}

						if f.Interface != nil {
							str := fmt.Sprintf("%v", f.Interface)
							scope.SetExtra(f.Key, str)
							continue
						}

						if f.String != "" {
							scope.SetExtra(f.Key, f.String)
							continue
						}

						if f.Type == zapcore.Float64Type {
							scope.SetExtra(f.Key, math.Float64frombits(uint64(f.Integer)))
							continue
						}

						scope.SetExtra(f.Key, f.Integer)
					}
				})
				sentry.CaptureException(fmt.Errorf("%s: %s", entry.Message, err))
				sentry.PopScope()
			}
		}
	}
}
