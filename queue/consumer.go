package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/henrywhitaker3/windowframe/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Message struct {
	Type    Task
	Payload []byte
}

type QueueConsumer interface {
	Consume(context.Context, HandlerFunc) error
	Close(ctx context.Context) error
}

type Consumer struct {
	c   QueueConsumer
	obs *Observer

	mu        *sync.Mutex
	handlers  map[Task]HandlerFunc
	shutdowns []ShutdownFunc
}

type ConsumerOpts struct {
	Consumer QueueConsumer
	Observer *Observer
}

func NewConsumer(opts ConsumerOpts) *Consumer {
	obs := opts.Observer
	if obs == nil {
		obs = NewObserver(ObserverOpts{})
	}
	return &Consumer{
		c:         opts.Consumer,
		obs:       obs,
		mu:        &sync.Mutex{},
		handlers:  map[Task]HandlerFunc{},
		shutdowns: []ShutdownFunc{},
	}
}

// Consume from the queue. Blocking.
func (c *Consumer) Consume(ctx context.Context) error {
	return c.c.Consume(ctx, c.handler)
}

// Handler passed to the queue consumer implemenation to wrap metrics and error handling.
func (c *Consumer) handler(ctx context.Context, payload []byte) error {
	ctx, span := tracing.NewSpan(ctx, "HandleTask", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	job := job{}
	if err := Unmarshal(payload, &job); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}
	span.SetAttributes(attribute.String("task", string(job.Task)))

	start := time.Now()
	handler, ok := c.handlers[job.Task]
	if !ok {
		err := fmt.Errorf("no handler registered for task: %w", ErrDeadLetter)
		c.obs.observeError(ctx, job.Task, start, err)
		return err
	}

	err := handler(ctx, job.Payload)
	if err != nil {
		c.obs.observeError(ctx, job.Task, start, err)
		return err
	}

	c.obs.observeSuccess(ctx, job.Task, start)
	return nil
}

func (c *Consumer) RegisterShutdown(f ShutdownFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.shutdowns = append(c.shutdowns, f)
}

func (c *Consumer) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, f := range c.shutdowns {
		if err := f(ctx); err != nil {
			return err
		}
	}

	if err := c.c.Close(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Consumer) RegisterHandler(kind Task, h HandlerFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handlers[kind] = h
}

func (c *Consumer) RegisterMetrics(reg prometheus.Registerer) {
}
