package log

import "context"

var DefaultContext FieldContextProvider = &MapContext{}

type FieldContextProvider interface {
	WithFields(context.Context, map[string]interface{}) context.Context
	ContextCollector
}

// WrappedContext is both a context and a logger, allowing either syntax
// log.WithField(ctx, "key", "val").Debug()
// or
// ctx = log.WithField(ctx, "key", "val")
type WrappedContext struct {
	context.Context
}

func (ctx WrappedContext) Debug(msg string) {
	Debug(ctx, msg)
}

func (ctx WrappedContext) Info(msg string) {
	Info(ctx, msg)
}

func (ctx WrappedContext) Error(msg string) {
	Error(ctx, msg)
}

func WithFields(ctx context.Context, fields map[string]interface{}) *WrappedContext {
	return &WrappedContext{
		Context: DefaultContext.WithFields(ctx, fields),
	}
}

func WithField(ctx context.Context, key string, value interface{}) *WrappedContext {
	return WithFields(ctx, map[string]interface{}{key: value})
}

func WithError(ctx context.Context, err error) *WrappedContext {
	return WithField(ctx, "error", err.Error())
}

type MapContext struct{}

var simpleContextKey = MapContext{}

func (sc MapContext) WithFields(parent context.Context, fields map[string]interface{}) context.Context {
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

func (sc MapContext) LogFieldsFromContext(ctx context.Context) map[string]interface{} {
	values, ok := ctx.Value(simpleContextKey).(map[string]interface{})
	if !ok {
		values = map[string]interface{}{}
	}

	return values
}
