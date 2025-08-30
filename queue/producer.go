package queue

import (
	"context"
	"fmt"

	"github.com/henrywhitaker3/windowframe/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type QueueProducer interface {
	Push(context.Context, Task, []byte, ...Option) error
	Close(context.Context) error
}

type Producer struct {
	p QueueProducer

	obs *Observer
}

type ProducerOpts struct {
	Producer QueueProducer
	Observer *Observer
}

func NewProducer(opts ProducerOpts) *Producer {
	obs := opts.Observer
	if obs == nil {
		obs = NewObserver(ObserverOpts{})
	}
	return &Producer{
		p:   opts.Producer,
		obs: obs,
	}
}

func (p *Producer) Push(
	ctx context.Context,
	is Task,
	payload any,
	opts ...Option,
) error {
	ctx, span := tracing.NewSpan(
		ctx,
		"PushTask",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String("task", string(is))),
	)
	defer span.End()

	job, err := newJob(is, payload)
	if err != nil {
		return fmt.Errorf("create job payload: %w", err)
	}
	by, err := Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}

	if err := p.p.Push(ctx, is, by, opts...); err != nil {
		p.obs.observerJobPushedError(ctx, is, opts, err)
		return err
	}
	p.obs.observerJobPushed(ctx, is, opts)
	return nil
}

func (p *Producer) Close(ctx context.Context) error {
	return p.p.Close(ctx)
}
