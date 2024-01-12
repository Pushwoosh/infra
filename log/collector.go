package infralog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// handlers is a map of handlers for different levels
var handlers []func(entry *LogEntry)

func Debug(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.DebugLevel, Message: message, Fields: fields})
}

func Info(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.InfoLevel, Message: message, Fields: fields})
}

func Warn(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.WarnLevel, Message: message, Fields: fields})
}

func Error(message string, fields ...zap.Field) {
	handleEntry(&LogEntry{Level: zapcore.ErrorLevel, Message: message, Fields: fields})
}

func Fatal(message string, fields ...zap.Field) {
	entry := &LogEntry{Level: zapcore.FatalLevel, Message: message, Fields: fields}
	handleEntry(entry)

	// in case if there are no fatal handlers, we exit with panic.
	// should be unreachable
	panic(entry)
}

func RegisterLogHandler(handler func(entry *LogEntry)) {
	// add handler to the beginning of the handlers list to
	// make it the first one to be called
	handlers = append([]func(entry *LogEntry){handler}, handlers...)
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
