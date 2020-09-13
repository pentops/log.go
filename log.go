// Package log exposes a minimal interface for structured logging. It supports
// log key/value pairs passed through context
package log

import (
	"context"
	"fmt"
	"os"
)

type Logger interface {
	Debug(context.Context, string)
	Info(context.Context, string)
	Error(context.Context, string)
}

var DefaultLogger Logger = SimpleLogger{
	Output:  os.Stderr,
	Context: DefaultContext,
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

var DefaultContext ContextProvider = &SimpleContext{}

// WrappedContext is both a context and a logger, allowing either syntax
// log.WithField(ctx, "key", "val").Debug()
// or
// ctx = log.WithField(ctx, "key", "val")
type WrappedContext struct {
	context.Context
	Logger Logger
}

func (ctx WrappedContext) Debug(msg string) {
	ctx.Logger.Debug(ctx, msg)
}

func (ctx WrappedContext) Info(msg string) {
	ctx.Logger.Info(ctx, msg)
}

func (ctx WrappedContext) Error(msg string) {
	ctx.Logger.Error(ctx, msg)
}

func WithFields(ctx context.Context, fields map[string]interface{}) *WrappedContext {
	return &WrappedContext{
		Context: DefaultContext.WithFields(ctx, fields),
		Logger:  DefaultLogger,
	}
}

func WithField(ctx context.Context, key string, value interface{}) *WrappedContext {
	return WithFields(ctx, map[string]interface{}{key: value})
}

func WithError(ctx context.Context, err error) *WrappedContext {
	return WithField(ctx, "error", err.Error())
}

func WithTrace(ctx context.Context, value string) *WrappedContext {
	return &WrappedContext{
		Context: DefaultContext.WithTrace(ctx, value),
		Logger:  DefaultLogger,
	}
}
