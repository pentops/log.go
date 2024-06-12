// Package log exposes a minimal interface for structured logging. It supports
// log key/value pairs passed through context
package log

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/exp/slog"
)

type Logger interface {
	SetLevel(slog.Level)

	Debug(context.Context, string)
	Info(context.Context, string)
	Error(context.Context, string)
	Warn(context.Context, string)

	AddCollector(ContextCollector)

	ErrorContext(ctx context.Context, msg string, args ...any)
}

var DefaultLogger Logger

func init() {

	logFormat := os.Getenv("LOG_FORMAT")
	var formatter LogFunc
	switch logFormat {
	case "pretty":
		formatter = PrettyLog(os.Stderr, SkipFields("version", "app"))
	default: // json and not set
		formatter = JSONLog(os.Stderr)
	}

	DefaultLogger = NewCallbackLogger(formatter)

	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		DefaultLogger.SetLevel(slog.LevelDebug)
	case "info":
		DefaultLogger.SetLevel(slog.LevelInfo)
	case "warn":
		DefaultLogger.SetLevel(slog.LevelWarn)
	case "error":
		DefaultLogger.SetLevel(slog.LevelError)
	default:
		DefaultLogger.SetLevel(slog.LevelInfo)
	}

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

func Warn(ctx context.Context, msg string) {
	DefaultLogger.Warn(ctx, msg)
}

func Warnf(ctx context.Context, msg string, params ...interface{}) {
	DefaultLogger.Warn(ctx, fmt.Sprintf(msg, params...))
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
	Level      slog.Level
	Callback   LogFunc
	Collectors []ContextCollector
}

func NewCallbackLogger(callback LogFunc) *CallbackLogger {
	return &CallbackLogger{
		Callback:   callback,
		Collectors: []ContextCollector{DefaultContext, DefaultTrace},
	}
}

func (sl *CallbackLogger) SetLevel(level slog.Level) {
	sl.Level = level
}

func (sl CallbackLogger) Debug(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelDebug, msg)
}

func (sl CallbackLogger) Info(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelInfo, msg)
}

func (sl CallbackLogger) Warn(ctx context.Context, msg string) {
	sl.log(ctx, slog.LevelWarn, msg)
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
	if level < sl.Level {
		return
	}

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
	if level < sl.Level {
		return
	}
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
	out.Write(append(logLine, '\n')) // nolint: errcheck
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

type loggerOptions struct {
	skipFields map[string]struct{}
}

type LoggerOption func(*loggerOptions)

func SkipFields(fields ...string) LoggerOption {
	return func(o *loggerOptions) {
		if o.skipFields == nil {
			o.skipFields = map[string]struct{}{}
		}
		for _, field := range fields {
			o.skipFields[field] = struct{}{}
		}
	}
}

func PrettyLog(out io.Writer, optionFuncs ...LoggerOption) LogFunc {
	var levelColors = map[string]color.Attribute{
		"debug": color.FgBlue,
		"info":  color.FgGreen,
		"warn":  color.FgYellow,
		"error": color.FgRed,
	}

	options := &loggerOptions{}

	for _, f := range optionFuncs {
		f(options)
	}

	return func(level string, msg string, fields map[string]interface{}) {
		whichColor, ok := levelColors[strings.ToLower(level)]
		if !ok {
			whichColor = color.FgWhite
		}

		levelColor := color.New(whichColor).SprintFunc()
		fmt.Fprintf(out, "%s: %s\n", levelColor(level), msg)

		for k, v := range fields {
			if _, skip := options.skipFields[k]; skip {
				continue
			}

			switch v.(type) {
			case string, int, int64, int32, float64, bool:
				fmt.Fprintf(out, "  | %s: %v\n", k, v)
			default:
				nice, _ := json.MarshalIndent(v, "  |  ", "  ")
				fmt.Fprintf(out, "  | %s: %s\n", k, string(nice))
			}
		}

	}
}
