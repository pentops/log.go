package log

import (
	"context"
	"io"
)

type TraceContextProvider interface {
	WithTrace(context.Context, string) context.Context
	FromContext(context.Context) string
	ContextCollector
}

var DefaultTrace TraceContextProvider = TraceContext{}

type logFormatter func(io.Writer, logEntry)

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

func (sc TraceContext) LogFieldsFromContext(ctx context.Context) map[string]interface{} {
	val, ok := ctx.Value(simpleTraceKey).(string)
	if !ok {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"trace": val,
	}
}
