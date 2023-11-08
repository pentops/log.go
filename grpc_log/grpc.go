package grpc_log

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

		logCtx := logContextProvider.WithFields(newCtx, nil)

		logger.Info(logCtx, "GRPC Handler Begin")

		var resp interface{}
		var mainError error
		func() {
			defer func() {
				if err := recover(); err != nil {
					logPanic(ctx, logContextProvider, err, logger)
					mainError = status.Error(codes.Internal, "Internal Error")
				}
			}()
			resp, mainError = handler(newCtx, req)
		}()

		if o.shouldLogBody(info.FullMethod, mainError) {
			logCtx = logContextProvider.WithFields(logCtx, map[string]interface{}{
				"requestBody": logBody(req),
			})
		}

		if !o.shouldLog(info.FullMethod, mainError) {
			return resp, mainError
		}

		logCtx = logContextProvider.WithFields(logCtx, map[string]interface{}{
			"durationSeconds": float32(time.Since(startTime).Nanoseconds()/1000) / 1000000,
			"code":            o.codeFunc(mainError),
		})

		if mainError != nil {
			logCtx = logContextProvider.WithFields(logCtx, map[string]interface{}{
				"error": mainError.Error(),
			})
		}

		logger.Info(logCtx, "GRPC Handler Complete")
		return resp, mainError
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

func logPanic(ctx context.Context, logContextProvider FieldContext, panicString interface{}, logger Logger) {
	into := make([]byte, 2048)
	runtime.Stack(into, false)

	stack := strings.Split(string(into), "\n")
	if len(stack) > 5 {
		// cut off the useless lines from panic to here
		stack = stack[5:]
	}
	for i, line := range stack {
		// tabs don't work well in JSON format
		stack[i] = strings.Replace(line, "\t", "    ", 1)
	}

	newCtx := logContextProvider.WithFields(ctx, map[string]interface{}{
		"error": panicString,
		"stack": stack,
	})

	logger.Info(newCtx, "GRPC Handler Panic")
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
