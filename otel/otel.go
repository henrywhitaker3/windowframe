// Package otel
package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type Opts struct {
	Tracing    tracing.TracingOpts
	Registerer prometheus.Registerer
	Version    string
}

func Init(opts Opts) (context.CancelFunc, error) {
	tracer, err := tracing.InitTracer(opts.Tracing, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("init otel tracer: %w", err)
	}

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(opts.Registerer),
	)
	if err != nil {
		return nil, fmt.Errorf("init otel prom exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(provider)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_ = tracer.Shutdown(ctx)
		_ = provider.Shutdown(ctx)
	}, nil
}
