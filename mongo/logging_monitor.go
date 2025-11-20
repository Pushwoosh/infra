package mongo

import (
	"context"

	"github.com/pushwoosh/infra/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.uber.org/zap"
)

// StringifyBSON converts arbitrary BSON to json-like string.
// it returns empty string on any error.
func StringifyBSON(val interface{}) string {
	// Q: wtf is canonical?
	// A: canonical flag makes marshaller to use extended json format like { "$numberLong": "123" }
	ret, err := bson.MarshalExtJSON(val, false, false)
	if err != nil {
		return ""
	}

	return string(ret)
}

// getFinishedLogger returns logger with common fields for succeeded and failed events
func getLogFields(e event.CommandFinishedEvent, origin *event.CommandStartedEvent) []zap.Field {
	return []zap.Field{
		zap.String("database", origin.DatabaseName),
		zap.String("command_name", e.CommandName),
		zap.Int64("request_id", e.RequestID),
		zap.String("connection_id", e.ConnectionID),
		zap.String("query", StringifyBSON(origin.Command)),
		zap.Duration("duration", e.Duration),
	}
}

// logSucceededEvent logs succeeded query
func logSucceededEvent(e *event.CommandSucceededEvent, origin *event.CommandStartedEvent) {
	fields := getLogFields(e.CommandFinishedEvent, origin)
	infralog.Debug("query succeeded", fields...)
}

// logFailedEvent logs failed query
func logFailedEvent(e *event.CommandFailedEvent, origin *event.CommandStartedEvent) {
	fields := getLogFields(e.CommandFinishedEvent, origin)
	infralog.Debug("query failed", append(fields, zap.Error(e.Failure))...)
}

func initLoggingMonitor(b *monitorBuilder, cfg *QueryLoggingConfig) {
	if cfg == nil || (!cfg.All && !cfg.Slow) {
		return
	}

	b.Add(
		func(ctx context.Context, startedEvent *event.CommandStartedEvent) {},
		func(ctx context.Context, startedEvent *event.CommandStartedEvent, succeededEvent *event.CommandSucceededEvent) {
			if startedEvent == nil {
				return
			}

			if cfg.All || (cfg.Slow && succeededEvent.Duration > cfg.SlowThreshold) {
				logSucceededEvent(succeededEvent, startedEvent)
			}
		},
		func(ctx context.Context, startedEvent *event.CommandStartedEvent, failedEvent *event.CommandFailedEvent) {
			if startedEvent == nil {
				return
			}

			if cfg.All || (cfg.Slow && failedEvent.Duration > cfg.SlowThreshold) {
				logFailedEvent(failedEvent, startedEvent)
			}
		},
	)
}
