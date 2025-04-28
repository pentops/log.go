package log

import (
	"context"
	"log/slog"
)

type TraceContextProvider interface {
	WithTrace(context.Context, string) context.Context
	FromContext(context.Context) string
	ContextCollector
}

var DefaultTrace TraceContextProvider = TraceContext{}

type TraceContext struct{}

var simpleTraceKey = TraceContext{}

func (sc TraceContext) WithTrace(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, simpleTraceKey, value)
}

func (sc TraceContext) FromContext(ctx context.Context) string {
	val, ok := ctx.Value(simpleTraceKey).(string)
	if !ok {
		return ""
	}
	return val
}

func (sc TraceContext) LogFieldsFromContext(ctx context.Context) []slog.Attr {
	val, ok := ctx.Value(simpleTraceKey).(string)
	if !ok {
		return []slog.Attr{}
	}
	return []slog.Attr{
		slog.String("trace", val),
	}
}
