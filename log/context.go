package log

import (
	"context"
	"log/slog"
	"sort"
)

type ContextCollector interface {
	LogFieldsFromContext(context.Context) []slog.Attr
}

var DefaultContext FieldContextProvider = &MapContext{}

type FieldContextProvider interface {
	WithAttrs(context.Context, []slog.Attr) context.Context
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

func (ctx WrappedContext) Warn(msg string) {
	Warn(ctx, msg)
}

func (ctx WrappedContext) Error(msg string) {
	Error(ctx, msg)
}

func WithFields(ctx context.Context, args ...any) *WrappedContext {
	attrs := collectArgs(args...)
	return &WrappedContext{
		Context: DefaultContext.WithAttrs(ctx, attrs),
	}
}

func WithField(ctx context.Context, args ...any) *WrappedContext {
	return WithFields(ctx, args...)
}

func WithError(ctx context.Context, err error) *WrappedContext {
	return WithField(ctx, "error", err.Error())
}

type MapContext struct{}

var simpleContextKey = MapContext{}

func (sc MapContext) WithAttrs(parent context.Context, attrs []slog.Attr) context.Context {
	existing, ok := parent.Value(simpleContextKey).([]slog.Attr)
	if !ok {
		return context.WithValue(parent, simpleContextKey, attrs)
	}

	// merge the new attributes into the existing ones
	// If an attribute with the same key exists, it will be replaced
	// inline, in the same order as the existing attribute, otherwise it is
	// appended to the end of the list.

	newAttrKeys := map[string]slog.Attr{}
	for _, attr := range attrs {
		newAttrKeys[attr.Key] = attr
	}

	newAttrs := make([]slog.Attr, 0, len(existing)+len(attrs))

	for _, attr := range existing {
		if _, exists := newAttrKeys[attr.Key]; !exists {
			newAttrs = append(newAttrs, attr)
		} else {
			newAttrs = append(newAttrs, newAttrKeys[attr.Key])
			delete(newAttrKeys, attr.Key)
		}

	}

	for _, attr := range attrs {
		if _, exists := newAttrKeys[attr.Key]; exists {
			newAttrs = append(newAttrs, attr)
		}
	}

	return context.WithValue(parent, simpleContextKey, newAttrs)
}

func (sc MapContext) LogFieldsFromContext(ctx context.Context) []slog.Attr {
	values, ok := ctx.Value(simpleContextKey).([]slog.Attr)
	if !ok {
		values = []slog.Attr{}
	}

	return values
}

func mapToAttrs(fields map[string]any) []slog.Attr {
	keys := []string{}
	for k := range fields {
		keys = append(keys, k)
	}
	sort.StringSlice(keys).Sort()

	attrs := make([]slog.Attr, len(keys))
	for _, k := range keys {
		attrs = append(attrs, slog.Any(k, fields[k]))
	}
	return attrs
}

func collectArgs(args ...any) []slog.Attr {
	var attrs []slog.Attr
	var nextAttrs []slog.Attr
	for len(args) > 0 {
		nextAttrs, args = shiftNextArg(args)
		attrs = append(attrs, nextAttrs...)
	}
	return attrs
}

const badKey = "!BADKEY"

func shiftNextArg(args []any) ([]slog.Attr, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return []slog.Attr{slog.String(badKey, x)}, nil
		}
		return []slog.Attr{slog.Any(x, args[1])}, args[2:]

	case slog.Attr:
		return []slog.Attr{x}, args[1:]

	case map[string]any:
		return mapToAttrs(x), args[1:]

	default:
		return []slog.Attr{slog.Any(badKey, x)}, args[1:]
	}
}
