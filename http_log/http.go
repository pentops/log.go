package http_log

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type FieldContext interface {
	WithFields(context.Context, map[string]interface{}) context.Context
}

type TraceContext interface {
	WithTrace(context.Context, string) context.Context
}

type Logger interface {
	Info(context.Context, string)
}

func Middleware(
	logContextProvider FieldContext,
	traceContextProvider TraceContext,
	logger Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			trace := req.Header.Get("x-trace")
			if trace == "" {
				trace = uuid.New().String()
			}

			// Respond with the trace header, as specified or created
			w.Header().Set("x-trace", trace)

			// Hack it so that the x-trace header is sent out in gRPC requests
			req.Header.Set("Grpc-Metadata-x-trace", trace)

			ctx := req.Context()
			ctx = logContextProvider.WithFields(ctx, map[string]interface{}{
				"method":   req.Method,
				"path":     req.URL.Path,
				"protocol": req.Proto,
				"trace":    trace,
			})
			req = req.WithContext(ctx)
			logger.Info(ctx, "Request")
			begin := time.Now()
			ss := &httpResponseStatusSpy{
				ResponseWriter: w,
				status:         http.StatusOK,
			}
			next.ServeHTTP(ss, req)
			ctx = logContextProvider.WithFields(ctx, map[string]interface{}{
				"method":     req.Method,
				"path":       req.URL.Path,
				"protocol":   req.Proto,
				"status":     ss.status,
				"durationMS": time.Since(begin).Milliseconds(),
			})
			logger.Info(ctx, "Response")
		})
	}
}

type httpResponseStatusSpy struct {
	http.ResponseWriter
	status int
}

func (s *httpResponseStatusSpy) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}
