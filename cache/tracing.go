package cache

import (
	"context"
	"fmt"

	"github.com/henrywhitaker3/windowframe/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func newTrace[T comparable](ctx context.Context, op string, key T) (context.Context, trace.Span) {
	return tracing.NewSpan(
		ctx,
		op,
		trace.WithAttributes(attribute.String("key", fmt.Sprintf("%v", key))),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}
