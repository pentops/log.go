// Package log exposes a minimal interface for structured logging. It supports
// log key/value pairs passed through context
package log

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/exp/slog"
)

type Logger interface {
	Debug(context.Context, string)
	Info(context.Context, string)
	Error(context.Context, string)
	AddCollector(ContextCollector)

	ErrorContext(ctx context.Context, msg string, args ...any)
}

var DefaultLogger Logger = NewCallbackLogger(JSONLog(os.Stderr))

func Debug(ctx context.Context, msg string) {
	DefaultLogger.Debug(ctx, msg)
}

func Debugf(ctx context.Context, msg string, params ...interface{}) {
	DefaultLogger.Debug(ctx, fmt.Sprintf(msg, params...))
}

func Info(ctx context.Context, msg string) {
	DefaultLogger.Info(ctx, msg)
}

func Infof(ctx context.Context, msg string, params ...interface{}) {
	DefaultLogger.Info(ctx, fmt.Sprintf(msg, params...))
}

func Error(ctx context.Context, msg string) {
	DefaultLogger.Error(ctx, msg)
}

func Errorf(ctx context.Context, msg string, params ...interface{}) {
	DefaultLogger.Error(ctx, fmt.Sprintf(msg, params...))
}

// Fatal logs, then causes the current program to exit status 1
// The program terminates immediately; deferred functions are not run.
func Fatal(ctx context.Context, msg string) {
	DefaultLogger.Error(ctx, msg)
	os.Exit(1)
}

// Fatalf logs, then causes the current program to exit status 1
// The program terminates immediately; deferred functions are not run.
func Fatalf(ctx context.Context, msg string, params ...interface{}) {
	Fatal(ctx, fmt.Sprintf(msg, params...))
}

type LogFunc func(level string, message string, fields map[string]interface{})

type CallbackLogger struct {
	Callback   LogFunc
	Collectors []ContextCollector
}

func NewCallbackLogger(callback LogFunc) *CallbackLogger {
	return &CallbackLogger{
		Callback:   callback,
		Collectors: []ContextCollector{DefaultContext, DefaultTrace},
	}
}

func (sl CallbackLogger) Debug(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelDebug, msg)
}
func (sl CallbackLogger) Info(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelInfo, msg)
}
func (sl CallbackLogger) Error(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelError, msg)
}
func (sl *CallbackLogger) AddCollector(collector ContextCollector) {
	sl.Collectors = append(sl.Collectors, collector)
}

func (sl CallbackLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	sl.slog(ctx, slog.LevelInfo, msg, args)
}

func (sl CallbackLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	sl.slog(ctx, slog.LevelDebug, msg, args)
}

func (sl CallbackLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	sl.slog(ctx, slog.LevelError, msg, args)
}

func (sl CallbackLogger) slog(ctx context.Context, level slog.Level, msg string, args []any) {
	fields := sl.extractFields(ctx)

	// Using record to extract the args into a map
	record := slog.NewRecord(time.Time{}, level, msg, 0)
	record.Add(args...)
	record.Attrs(func(attr slog.Attr) bool {
		fields[attr.Key] = attr.Value
		return true
	})
	sl.Callback(level.String(), msg, fields)
}

func (sl CallbackLogger) extractFields(ctx context.Context) map[string]interface{} {
	fields := map[string]interface{}{}
	for _, cb := range sl.Collectors {
		for k, v := range cb.LogFieldsFromContext(ctx) {
			fields[k] = v
		}
	}
	return fields
}

func (sl CallbackLogger) log(ctx context.Context, level slog.Level, msg string) {
	fields := sl.extractFields(ctx)
	sl.Callback(level.String(), msg, fields)
}

type ContextCollector interface {
	LogFieldsFromContext(context.Context) map[string]interface{}
}

type logEntry struct {
	Level   string                 `json:"level"`
	Time    time.Time              `json:"time"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields"`
}

func jsonFormatter(out io.Writer, entry logEntry) {
	logLine, err := json.Marshal(entry)
	if err != nil {
		logLine, _ = json.Marshal(logEntry{
			Message: entry.Message,
			Time:    entry.Time,
			Level:   entry.Level,
			// Not passing through fields which is where the error would have
			// been
		})
	}
	out.Write(append(logLine, '\n'))
}

func SimplifyFields(fields map[string]interface{}) map[string]interface{} {
	simplified := map[string]interface{}{}
	for k, v := range fields {
		if err, ok := v.(error); ok {
			v = err.Error()
		} else if err, ok := v.(fmt.Stringer); ok {
			v = err.String()
		}

		simplified[k] = v
	}
	return simplified
}

func JSONLog(out io.Writer) LogFunc {
	return func(level string, msg string, fields map[string]interface{}) {

		jsonFormatter(out, logEntry{
			Level:   level,
			Time:    time.Now(),
			Message: msg,
			Fields:  SimplifyFields(fields),
		})
	}
}
