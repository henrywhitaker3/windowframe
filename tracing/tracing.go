package tracing

import (
	"context"
	"os"
	"os/user"
	"strconv"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	otrace "go.opentelemetry.io/otel/trace"
)

var (
	name           = "not set"
	TracerProvider otrace.TracerProvider
)

type TracingOpts struct {
	ServiceName      string
	Endpoint         string
	Environment      string
	SampleRate       float64
	ProfilingEnabled bool
}

func InitTracer(conf TracingOpts, version string) (*trace.TracerProvider, error) {
	name = conf.ServiceName
	var exporter trace.SpanExporter
	var err error
	if conf.Endpoint == "stdout" {
		exporter, err = stdout.New(stdout.WithPrettyPrint())
	} else if conf.Endpoint != "" {
		exporter, err = otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpointURL(conf.Endpoint),
		)
	} else {
		// Without explicit options, the exporter uses the standard OTEL_EXPORTER_OTLP_*
		// environment variables (including OTEL_EXPORTER_OTLP[_TRACES]_ENDPOINT).
		exporter, err = otlptracehttp.New(context.Background())
	}
	if err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{}
	if conf.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(conf.ServiceName))
	}
	if version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	if conf.Environment != "" {
		attrs = append(attrs, attribute.String("environment", conf.Environment))
	}

	resourceOptions := []resource.Option{
		resource.WithContainer(),
		resource.WithOS(),
		resource.WithHost(),
	}
	resourceOptions = append(resourceOptions, processResourceOptions()...)
	resourceOptions = append(resourceOptions,
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	)
	res, err := resource.New(context.Background(), resourceOptions...)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.TraceIDRatioBased(conf.SampleRate)),
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)
	var wrapped otrace.TracerProvider = tp
	if conf.ProfilingEnabled {
		wrapped = otelpyroscope.NewTracerProvider(tp)
	}

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	otel.SetTracerProvider(wrapped)
	TracerProvider = wrapped
	return tp, nil
}

// processResourceOptions is equivalent to resource.WithProcess, except that
// process.owner falls back to the process UID when no passwd entry exists.
// This is common for non-root and scratch containers, where os/user.Current
// otherwise makes tracing initialization fail.
func processResourceOptions() []resource.Option {
	return []resource.Option{
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessCommandArgs(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithAttributes(processOwnerAttribute(user.Current, strconv.Itoa(os.Getuid()))),
	}
}

func processOwnerAttribute(currentUser func() (*user.User, error), fallback string) attribute.KeyValue {
	if current, err := currentUser(); err == nil && current.Username != "" {
		return semconv.ProcessOwner(current.Username)
	}
	return semconv.ProcessOwner(fallback)
}

func NewSpan(
	ctx context.Context,
	name string,
	opts ...otrace.SpanStartOption,
) (context.Context, otrace.Span) {
	if TracerProvider == nil {
		return otel.Tracer(name).Start(ctx, name, opts...)
	}
	return TracerProvider.Tracer(name).Start(ctx, name, opts...)
}

func AddString(ctx context.Context, key, value string) {
	if span := otrace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(attribute.String(key, value))
	}
}

func TraceID(ctx context.Context) string {
	span := otrace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
