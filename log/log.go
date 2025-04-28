// Package log exposes a minimal interface for structured logging. It supports
// log key/value pairs passed through context
package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
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

func Debugf(ctx context.Context, msg string, params ...any) {
	DefaultLogger.Debug(ctx, fmt.Sprintf(msg, params...))
}

func Info(ctx context.Context, msg string) {
	DefaultLogger.Info(ctx, msg)
}

func Infof(ctx context.Context, msg string, params ...any) {
	DefaultLogger.Info(ctx, fmt.Sprintf(msg, params...))
}

func Warn(ctx context.Context, msg string) {
	DefaultLogger.Warn(ctx, msg)
}

func Warnf(ctx context.Context, msg string, params ...any) {
	DefaultLogger.Warn(ctx, fmt.Sprintf(msg, params...))
}

func Error(ctx context.Context, msg string) {
	DefaultLogger.Error(ctx, msg)
}

func Errorf(ctx context.Context, msg string, params ...any) {
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
func Fatalf(ctx context.Context, msg string, params ...any) {
	Fatal(ctx, fmt.Sprintf(msg, params...))
}

type LogFunc func(level string, message string, attrs []slog.Attr)

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
		fields = append(fields, attr)
		return true
	})
	sl.Callback(level.String(), msg, fields)
}

func (sl CallbackLogger) extractFields(ctx context.Context) []slog.Attr {
	fields := []slog.Attr{}
	for _, cb := range sl.Collectors {
		fields = append(fields, cb.LogFieldsFromContext(ctx)...)
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

type TB interface {
	Logf(string, ...any)
}

func NewTestLogger(t TB) *CallbackLogger {
	ll := NewCallbackLogger(func(level string, msg string, fields []slog.Attr) {
		t.Logf("%s: %s", level, msg)
		for _, attr := range fields {
			k := attr.Key
			v := attr.Value.Any()
			t.Logf("  | %s: %v", k, v)
		}
	})
	ll.SetLevel(slog.LevelDebug)
	return ll
}

type logEntry struct {
	Level   string    `json:"level"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Fields  attrMap   `json:"fields"`
}

type attrMap []slog.Attr

func (aa attrMap) find(key string) (any, bool) {
	for _, kv := range aa {
		if kv.Key == key {
			return kv.Value.Any(), true
		}
	}
	return nil, false
}

func (aa attrMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for i, kv := range aa {
		if i != 0 {
			buf.WriteString(",")
		}
		// marshal key
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		// marshal value
		val, err := json.Marshal(kv.Value.Any())
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
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

func JSONLog(out io.Writer) LogFunc {
	return func(level string, msg string, attrs []slog.Attr) {
		jsonFormatter(out, logEntry{
			Level:   level,
			Time:    time.Now(),
			Message: msg,
			Fields:  attrMap(attrs),
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

	return func(level string, msg string, attrs []slog.Attr) {
		whichColor, ok := levelColors[strings.ToLower(level)]
		if !ok {
			whichColor = color.FgWhite
		}

		levelColor := color.New(whichColor).SprintFunc()
		fmt.Fprintf(out, "%s: %s\n", levelColor(level), msg)

		for _, attr := range attrs {
			k := attr.Key
			v := attr.Value.Any()
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
