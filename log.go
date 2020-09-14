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
)

type Logger interface {
	Debug(context.Context, string)
	Info(context.Context, string)
	Error(context.Context, string)
	AddCollector(ContextCollector)
}

var DefaultLogger Logger = &SimpleLogger{
	Output:     os.Stderr,
	Collectors: []ContextCollector{DefaultContext, DefaultTrace},
}

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

type SimpleLogger struct {
	Output     io.Writer
	Format     logFormatter
	Collectors []ContextCollector
}

type ContextCollector interface {
	LogFieldsFromContext(context.Context) map[string]interface{}
}

const (
	debugLevel = "DEBUG"
	infoLevel  = "INFO"
	errorLevel = "ERROR"
)

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

func (sl SimpleLogger) log(ctx context.Context, level string, msg string) {
	if sl.Format == nil {
		// lazy default
		sl.Format = jsonFormatter
	}

	fields := map[string]interface{}{}
	for _, cb := range sl.Collectors {
		for k, v := range cb.LogFieldsFromContext(ctx) {
			fields[k] = v
		}
	}

	sl.Format(sl.Output, logEntry{
		Level:   level,
		Time:    time.Now(),
		Message: msg,
		Fields:  fields,
	})
}

func (sl SimpleLogger) Debug(ctx context.Context, msg string) {
	sl.log(ctx, debugLevel, msg)
}
func (sl SimpleLogger) Info(ctx context.Context, msg string) {
	sl.log(ctx, infoLevel, msg)
}
func (sl SimpleLogger) Error(ctx context.Context, msg string) {
	sl.log(ctx, errorLevel, msg)
}
func (sl *SimpleLogger) AddCollector(collector ContextCollector) {
	sl.Collectors = append(sl.Collectors, collector)
}
