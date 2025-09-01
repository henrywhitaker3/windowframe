package queue

import (
	"context"

	"github.com/henrywhitaker3/windowframe/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type QueueProducer interface {
	Push(context.Context, Job) error
	Close(context.Context) error
}

type Producer struct {
	p QueueProducer

	obs *ProducerObserver
}

type ProducerOpts struct {
	Producer QueueProducer
	Observer *ProducerObserver
}

func NewProducer(opts ProducerOpts) *Producer {
	obs := opts.Observer
	if obs == nil {
		obs = NewProducerObserver(ProducerObserverOpts{})
	}
	return &Producer{
		p:   opts.Producer,
		obs: obs,
	}
}

func (p *Producer) Push(
	ctx context.Context,
	job Job,
) error {
	ctx, span := tracing.NewSpan(
		ctx,
		"PushTask",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String("task", string(job.Task))),
	)
	defer span.End()
	job.Options = append(job.Options, withID(job.ID))

	if err := p.p.Push(ctx, job); err != nil {
		p.obs.observerJobPushedError(ctx, job.Task, job.Options, err)
		return err
	}
	p.obs.observerJobPushed(ctx, job.Task, job.Options)
	return nil
}

func (p *Producer) Close(ctx context.Context) error {
	return p.p.Close(ctx)
}
