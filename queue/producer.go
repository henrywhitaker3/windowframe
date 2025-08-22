package queue

import (
	"context"
	"fmt"
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
