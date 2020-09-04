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
	ContextProvider
}

var DefaultLogger Logger = SimpleLogger{
	Output:          os.Stderr,
	ContextProvider: &simpleContext{},
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

var contextKey = struct{}{}

// WrappedContext is both a context and a logger, allowing either syntax convention,
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
		Context: DefaultLogger.WithFields(ctx, fields),
		Logger:  DefaultLogger,
	}
}

func WithField(ctx context.Context, key string, value interface{}) *WrappedContext {
	return WithFields(ctx, map[string]interface{}{key: value})
}

func WithError(ctx context.Context, err error) *WrappedContext {
	return WithField(ctx, "error", err.Error())
}
