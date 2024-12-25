package infralog

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogHandler is the callback function that can be used for handling log entries
type LogHandler func(entry *LogEntry)

// handlers is a map of handlers for different levels
var handlers []LogHandler

func Debug(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.DebugLevel, Message: message, Fields: fields})
}

func DebugCtx(ctx context.Context, message string, fields ...zap.Field) {
	ctxFields := fieldsFromContext(ctx)
	fields = append(ctxFields, fields...)
	handleEntry(&LogEntry{Level: zapcore.DebugLevel, Message: message, Fields: fields})
}

func Info(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.InfoLevel, Message: message, Fields: fields})
}

func InfoCtx(ctx context.Context, message string, fields ...zap.Field) {
	ctxFields := fieldsFromContext(ctx)
	fields = append(ctxFields, fields...)
	handleEntry(&LogEntry{Level: zapcore.InfoLevel, Message: message, Fields: fields})
}

func Warn(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.WarnLevel, Message: message, Fields: fields})
}

func WarnCtx(ctx context.Context, message string, fields ...zap.Field) {
	ctxFields := fieldsFromContext(ctx)
	fields = append(ctxFields, fields...)
	handleEntry(&LogEntry{Level: zapcore.WarnLevel, Message: message, Fields: fields})
}

func Error(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.ErrorLevel, Message: message, Fields: fields})
}

func ErrorCtx(ctx context.Context, message string, fields ...zap.Field) {
	ctxFields := fieldsFromContext(ctx)
	fields = append(ctxFields, fields...)
	handleEntry(&LogEntry{Level: zapcore.ErrorLevel, Message: message, Fields: fields})
}

func Fatal(message string, fields ...zap.Field) {
	entry := &LogEntry{Level: zapcore.FatalLevel, Message: message, Fields: fields}
	handleEntry(entry)

	// in case if there are no fatal handlers, we exit with panic.
	// should be unreachable
	panic(entry)
}

func FatalCtx(ctx context.Context, message string, fields ...zap.Field) {
	ctxFields := fieldsFromContext(ctx)
	fields = append(ctxFields, fields...)
	entry := &LogEntry{Level: zapcore.FatalLevel, Message: message, Fields: fields}
	handleEntry(entry)

	// in case if there are no fatal handlers, we exit with panic.
	// should be unreachable
	panic(entry)
}

type fieldsCtxKeyType string

const fieldsCtxKey fieldsCtxKeyType = "fields"

type logFields struct {
	fields []zap.Field
}

func fieldsFromContext(ctx context.Context) []zap.Field {
	if ctxFields, ok := ctx.Value(fieldsCtxKey).(*logFields); ok {
		return ctxFields.fields
	}
	return nil
}

func WithField(ctx context.Context, field zap.Field) context.Context {
	ctxFields, ok := ctx.Value(fieldsCtxKey).(*logFields)
	if !ok || ctxFields == nil {
		ctxFields = &logFields{}
		ctx = context.WithValue(ctx, fieldsCtxKey, ctxFields)
	}
	ctxFields.fields = append(ctxFields.fields, field)
	return ctx
}

func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	ctxFields := ctx.Value(fieldsCtxKey).(*logFields)
	if ctxFields == nil {
		ctxFields = &logFields{}
		ctx = context.WithValue(ctx, fieldsCtxKey, ctxFields)
	}
	ctxFields.fields = append(ctxFields.fields, fields...)
	return ctx
}

func RegisterLogHandler(handler LogHandler) {
	// add handler to the beginning of the handlers list to
	// make it the first one to be called
	handlers = append([]LogHandler{handler}, handlers...)
}

func handleEntry(entry *LogEntry) {
	for _, handler := range handlers {
		handler(entry)
	}
}

type LogEntry struct {
	Level   zapcore.Level
	Message string
	Fields  []zap.Field
}

// Error searches the first error in the list of entry's fields.
// Used to extract error from the log entry. We usually have only one error.
func (e *LogEntry) Error() error {
	for i := range e.Fields {
		if e.Fields[i].Type == zapcore.ErrorType {
			if err, ok := e.Fields[i].Interface.(error); ok {
				return err
			}
		}
	}
	return nil
}
