package otel_log

import (
	"context"

	"go.opentelemetry.io/otel/api/trace"
)

func Extractor(ctx context.Context, vals map[string]interface{}) {
	span := trace.SpanFromContext(ctx).SpanContext()
	if span.HasSpanID() {
		vals["span"] = span.SpanID
	}
	if span.HasTraceID() {
		vals["trace"] = span.TraceID
	}
}
