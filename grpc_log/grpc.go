package grpc_log

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"gopkg.daemonl.com/log"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
)

type options struct {
	shouldLog grpc_logging.Decider
	codeFunc  grpc_logging.ErrorToCode
}

type Option func(*options)

// WithDecider customizes the function for deciding if the gRPC interceptor logs should log.
func WithDecider(f grpc_logging.Decider) Option {
	return func(o *options) {
		o.shouldLog = f
	}
}

// WithCodes customizes the function for mapping errors to error codes.
func WithCodes(f grpc_logging.ErrorToCode) Option {
	return func(o *options) {
		o.codeFunc = f
	}
}

var defaultOptions = &options{
	shouldLog: grpc_logging.DefaultDeciderMethod,
	codeFunc:  grpc_logging.DefaultErrorToCode,
}

func evaluateServerOpt(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}

func UnaryServerInterceptor(logger log.Logger, options ...Option) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(options)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		logFields := map[string]interface{}{
			"method": info.FullMethod,
		}
		newCtx := logger.WithFields(ctx, logFields)
		resp, err := handler(newCtx, req)
		if !o.shouldLog(info.FullMethod, err) {
			return resp, err
		}

		logCtx := logger.WithFields(newCtx, map[string]interface{}{
			"duration": float32(time.Since(startTime).Nanoseconds()/1000) / 1000,
			"code":     o.codeFunc(err),
		})

		logger.Info(logCtx, "done")
		return resp, err
	}
}

func StreamServerInterceptor(logger log.Logger, options ...Option) grpc.StreamServerInterceptor {
	o := evaluateServerOpt(options)
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		logFields := map[string]interface{}{
			"method": info.FullMethod,
		}
		newCtx := logger.WithFields(stream.Context(), logFields)
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		err := handler(srv, wrapped)
		if !o.shouldLog(info.FullMethod, err) {
			return err
		}

		logCtx := logger.WithFields(newCtx, map[string]interface{}{
			"duration": float32(time.Since(startTime).Nanoseconds()/1000) / 1000,
			"code":     o.codeFunc(err),
		})

		logger.Info(logCtx, "done")
		return err
	}
}
