package log

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

type logFormatter func(io.Writer, logEntry)

type ExtractorFunc func(context.Context, map[string]interface{})

type simpleContext struct {
	extractors []ExtractorFunc
}

var simpleContextKey struct{}

func (sc simpleContext) WithFields(parent context.Context, fields map[string]interface{}) context.Context {
	existing, ok := parent.Value(simpleContextKey).(map[string]interface{})
	if !ok {
		return context.WithValue(parent, simpleContextKey, fields)
	}
	newMap := map[string]interface{}{}
	for k, v := range existing {
		newMap[k] = v
	}
	for k, v := range fields {
		newMap[k] = v
	}
	return context.WithValue(parent, simpleContextKey, newMap)
}

func (sc *simpleContext) AddExtractor(cb ExtractorFunc) {
	sc.extractors = append(sc.extractors, cb)
}

func (sc simpleContext) FromContext(ctx context.Context) map[string]interface{} {
	values, ok := ctx.Value(simpleContextKey).(map[string]interface{})
	if !ok {
		values = map[string]interface{}{}
	}

	if len(sc.extractors) > 0 {
		clone := map[string]interface{}{}
		for k, v := range values {
			clone[k] = v
		}
		values = clone

		for _, cb := range sc.extractors {
			cb(ctx, values)
		}
	}

	return values
}

type ContextProvider interface {
	FromContext(context.Context) map[string]interface{}
	WithFields(context.Context, map[string]interface{}) context.Context
	AddExtractor(ExtractorFunc)
}

type SimpleLogger struct {
	Output io.Writer
	Format logFormatter
	ContextProvider
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

func (sl SimpleLogger) log(ctx context.Context, level string, msg string) {
	if sl.Format == nil {
		// lazy default
		sl.Format = jsonFormatter
	}
	sl.Format(sl.Output, logEntry{
		Level:   level,
		Time:    time.Now(),
		Message: msg,
		Fields:  sl.FromContext(ctx),
	})
}

func jsonFormatter(out io.Writer, entry logEntry) {
	logLine, err := json.Marshal(entry)
	if err != nil {
		logLine, _ = json.Marshal(logEntry{
			Message: entry.Message,
			Time:    entry.Time,
			Level:   entry.Level,
			// Not passing through fields
		})
	}
	out.Write(append(logLine, '\n'))
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