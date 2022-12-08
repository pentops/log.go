package grpc_log

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
)

type options struct {
	shouldLog     grpc_logging.Decider
	shouldLogBody grpc_logging.Decider
	codeFunc      grpc_logging.ErrorToCode
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

// WithRequestBody customizes the function for deciding if the gRPC interceptor logs the request body.
func WithRequestBody(f grpc_logging.Decider) Option {
	return func(o *options) {
		o.shouldLogBody = f
	}
}

var defaultOptions = &options{
	shouldLog:     grpc_logging.DefaultDeciderMethod,
	shouldLogBody: grpc_logging.DefaultDeciderMethod,
	codeFunc:      grpc_logging.DefaultErrorToCode,
}

func evaluateServerOpt(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}

type FieldContext interface {
	WithFields(context.Context, map[string]interface{}) context.Context
}

type TraceContext interface {
	WithTrace(context.Context, string) context.Context
}

type Logger interface {
	Info(context.Context, string)
}

func UnaryServerInterceptor(
	logContextProvider FieldContext,
	traceContextProvider TraceContext,
	logger Logger,
	options ...Option,
) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(options)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		logFields := map[string]interface{}{
			"method": info.FullMethod,
		}
		newCtx := logContextProvider.WithFields(ctx, logFields)

		md, ok := metadata.FromIncomingContext(newCtx)
		if ok {
			traceHeaders := md.Get("x-trace")
			if len(traceHeaders) == 0 {
				traceHeaders = md.Get("x-request-id")
				if len(traceHeaders) == 0 {
					traceHeaders = []string{uuid.New().String()}
				}
			}
			traceHeader := traceHeaders[0]
			newCtx = traceContextProvider.WithTrace(newCtx, traceHeader)
			newCtx = metadata.AppendToOutgoingContext(newCtx, "x-trace", traceHeader)
		}

		defer logPanic(ctx, logContextProvider, logger)

		resp, err := handler(newCtx, req)
		if !o.shouldLog(info.FullMethod, err) {
			return resp, err
		}

		logCtx := logContextProvider.WithFields(newCtx, nil)

		if o.shouldLogBody(info.FullMethod, err) {
			logCtx = logContextProvider.WithFields(logCtx, map[string]interface{}{
				"body": logBody(req),
			})
		}

		logger.Info(logCtx, "GRPC Handler Begin")

		logCtx = logContextProvider.WithFields(newCtx, map[string]interface{}{
			"durationSeconds": float32(time.Since(startTime).Nanoseconds()/1000) / 1000000,
			"code":            o.codeFunc(err),
		})

		if err != nil {
			logCtx = logContextProvider.WithFields(logCtx, map[string]interface{}{
				"error": err.Error(),
			})
		}

		logger.Info(logCtx, "GRPC Handler Complete")
		return resp, err
	}
}

func logBody(msg interface{}) string {
	if p, ok := msg.(proto.Message); ok {
		msgBytes, err := protojson.Marshal(p)
		if err != nil {
			return fmt.Sprintf("Marshal Error: %s", err.Error())
		}
		return string(msgBytes)
	}
	return fmt.Sprintf("Non proto message of type %T", msg)
}

func logPanic(ctx context.Context, logContextProvider FieldContext, logger Logger) {
	if err := recover(); err != nil {
		newCtx := logContextProvider.WithFields(ctx, map[string]interface{}{
			"error": err,
		})

		logger.Info(newCtx, "GRPC Handler Begin & Panic")
	}
}

func StreamServerInterceptor(
	logContextProvider FieldContext,
	traceContextProvider TraceContext,
	logger Logger,
	options ...Option,
) grpc.StreamServerInterceptor {
	o := evaluateServerOpt(options)
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		logFields := map[string]interface{}{
			"method": info.FullMethod,
		}
		newCtx := logContextProvider.WithFields(stream.Context(), logFields)

		md, ok := metadata.FromIncomingContext(newCtx)
		if ok {
			traceHeader := md.Get("x-trace")
			if len(traceHeader) > 0 {
				traceHeader = []string{uuid.New().String()}
			}
			newCtx = traceContextProvider.WithTrace(newCtx, traceHeader[0])
			newCtx = metadata.AppendToOutgoingContext(newCtx, "x-trace", traceHeader[0])
		}

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		err := handler(srv, wrapped)
		if !o.shouldLog(info.FullMethod, err) {
			return err
		}

		logCtx := logContextProvider.WithFields(newCtx, map[string]interface{}{
			"duration": float32(time.Since(startTime).Nanoseconds()/1000) / 1000,
			"code":     o.codeFunc(err),
		})

		logger.Info(logCtx, "GRPC Stream Complete")
		return err
	}
}
