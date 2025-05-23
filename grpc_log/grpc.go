package grpc_log

import (
	"context"
	"fmt"
	"log/slog"
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
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

type options struct {
	shouldLogBody alwaysDecider
	codeFunc      grpc_logging.ErrorToCode
}

type alwaysDecider func(methodName string) bool

type Option func(*options)

// WithCodes customizes the function for mapping errors to error codes.
func WithCodes(f grpc_logging.ErrorToCode) Option {
	return func(o *options) {
		o.codeFunc = f
	}
}

// WithRequestBody customizes the function for deciding if the gRPC interceptor logs the request body.
func WithRequestBody(f alwaysDecider) Option {
	return func(o *options) {
		o.shouldLogBody = f
	}
}

var defaultOptions = &options{
	shouldLogBody: func(string) bool { return true },
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
	WithAttrs(context.Context, ...slog.Attr) context.Context
}

type TraceContext interface {
	WithTrace(context.Context, string) context.Context
}

type Logger interface {
	Info(context.Context, string)
	Error(context.Context, string)
	Debug(context.Context, string)
}

func UnaryServerInterceptor(
	logContextProvider FieldContext,
	traceContextProvider TraceContext,
	logger Logger,
	options ...Option,
) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(options)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startTime := time.Now()
		newCtx := logContextProvider.WithAttrs(ctx, slog.String("method", info.FullMethod))

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

		logCtx := logContextProvider.WithAttrs(newCtx) // empty clone

		if o.shouldLogBody(info.FullMethod) {
			subContext := logContextProvider.WithAttrs(logCtx, slog.Any("requestBody", logBody(req)))
			logger.Info(subContext, "GRPC Handler Begin")
		} else {
			logger.Info(logCtx, "GRPC Handler Begin")
		}

		var resp any
		var mainError error
		func() {
			defer func() {
				if err := recover(); err != nil {
					logPanic(logCtx, logContextProvider, err, logger)
					mainError = status.Error(codes.Internal, "Internal Error")
				}
			}()
			resp, mainError = handler(newCtx, req)
		}()

		logCtx = logContextProvider.WithAttrs(logCtx,
			slog.Float64("durationSeconds", float64(time.Since(startTime).Nanoseconds()/1000)/1000000),
			slog.String("code", o.codeFunc(mainError).String()),
		)

		if mainError != nil {
			logCtx = logContextProvider.WithAttrs(logCtx, slog.String("error", mainError.Error()))
			logger.Error(logCtx, "GRPC Handler Complete")
		} else {
			logger.Info(logCtx, "GRPC Handler Complete")
		}
		return resp, mainError
	}
}

func logBody(msg any) string {
	if p, ok := msg.(proto.Message); ok {
		msgBytes, err := protojson.Marshal(p)
		if err != nil {
			return fmt.Sprintf("Marshal Error: %s", err.Error())
		}
		return string(msgBytes)
	}
	return fmt.Sprintf("Non proto message of type %T", msg)
}

func logPanic(ctx context.Context, logContextProvider FieldContext, panicValue any, logger Logger) {
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

	newCtx := logContextProvider.WithAttrs(ctx,
		slog.Any("error", panicValue),
		slog.Any("stack", strings.Join(stack, "\n")),
	)

	logger.Error(newCtx, "GRPC Handler Panic")
}

func StreamServerInterceptor(
	logContextProvider FieldContext,
	traceContextProvider TraceContext,
	logger Logger,
	options ...Option,
) grpc.StreamServerInterceptor {
	o := evaluateServerOpt(options)
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		newCtx := logContextProvider.WithAttrs(stream.Context(), slog.String("method", info.FullMethod))

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

		logCtx := logContextProvider.WithAttrs(newCtx,
			slog.Float64("duration", float64(time.Since(startTime).Nanoseconds()/1000)/1000),
			slog.String("code", o.codeFunc(err).String()),
		)

		logger.Info(logCtx, "GRPC Stream Complete")
		return err
	}
}
