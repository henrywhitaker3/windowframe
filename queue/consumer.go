package queue

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/henrywhitaker3/flow"
	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Message interface {
	Job() Job

	// Acknowledge the message as processed
	Ack(context.Context) error
	// Cancel the message, will no be retried
	Cancel(context.Context) error
	// Put the message back onto the queue
	Nack(context.Context) error
	// Put the message on the deadletter queue
	Deadletter(context.Context) error
}

// ProgressReporter is optionally implemented by a Message when its driver
// needs to be told that a long-running job is still being worked on, to stop
// it being redelivered and processed a second time while the first attempt
// is still in flight.
type ProgressReporter interface {
	InProgress(context.Context) error
}

type QueueConsumer interface {
	Consume(context.Context, chan<- Message) error
	Close(ctx context.Context) error
}

type Consumer struct {
	c   QueueConsumer
	obs *ConsumerObserver

	opts ConsumerOpts

	mu        *sync.Mutex
	handlers  map[Task]HandlerFunc
	shutdowns []ShutdownFunc
}

type ConsumerOpts struct {
	Consumer QueueConsumer
	Observer *ConsumerObserver

	// The number of concurrent threads processing messages (default: num cpu)
	Concurrency int
	// Ack attaempts (default: 3)
	AckAttempts int
	// The time between each each attempt (default: 5ms)
	AckBackoff time.Duration
	// How often to tell the driver a long-running job is still in progress,
	// for drivers whose Message implements ProgressReporter (default: 10s)
	HeartbeatInterval time.Duration
}

func (c ConsumerOpts) withDefaults() ConsumerOpts {
	if c.Concurrency == 0 {
		c.Concurrency = runtime.NumCPU()
	}
	if c.AckAttempts == 0 {
		c.AckAttempts = 3
	}
	if c.AckBackoff == 0 {
		c.AckBackoff = time.Millisecond * 5
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = time.Second * 10
	}
	return c
}

func NewConsumer(opts ConsumerOpts) *Consumer {
	obs := opts.Observer
	if obs == nil {
		obs = NewConsumerObserver(ConsumerObserverOpts{})
	}
	return &Consumer{
		c:         opts.Consumer,
		obs:       obs,
		opts:      opts.withDefaults(),
		mu:        &sync.Mutex{},
		handlers:  map[Task]HandlerFunc{},
		shutdowns: []ShutdownFunc{},
	}
}

// Consume from the queue.
// Blocks until the context is cancelled. When this happens, the Close(ctx) method is called
// on the consumer driver.
func (c *Consumer) Consume(ctx context.Context) error {
	messages := make(chan Message, c.opts.Concurrency*2)
	if err := c.c.Consume(ctx, messages); err != nil {
		return fmt.Errorf("consume from driver: %w", err)
	}

	for range c.opts.Concurrency {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-messages:
					_ = c.handler(ctx, msg)
				}
			}
		}()
	}

	<-ctx.Done()
	if err := c.c.Close(context.Background()); err != nil {
		return fmt.Errorf("close consumer driver: %w", err)
	}

	if len(messages) > 0 {
		size := len(messages)
		for range size {
			_ = c.handler(context.Background(), <-messages)
		}
	}

	return nil
}

// Handler passed to the queue consumer implemenation to wrap metrics and error handling.
func (c *Consumer) handler(ctx context.Context, msg Message) error {
	ctx, span := tracing.NewSpan(ctx, "HandleTask", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	ctx = c.obs.observeStart(ctx, msg.Job())

	span.SetAttributes(attribute.String("task", string(msg.Job().Task)))

	start := time.Now()
	handler, ok := c.handlers[msg.Job().Task]
	if !ok {
		err := fmt.Errorf("no handler registered for task: %w", ErrDeadLetter)
		c.obs.observeError(ctx, msg.Job(), start, err)
		return msg.Deadletter(ctx)
	}

	if pr, ok := msg.(ProgressReporter); ok {
		hbCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go c.heartbeat(hbCtx, pr)
	}

	err := handler(ctx, msg.Job().Payload)
	if err != nil {
		c.obs.observeError(ctx, msg.Job(), start, err)
		if errors.Is(err, ErrSkipRetry) {
			return msg.Cancel(ctx)
		}
		if errors.Is(err, ErrDeadLetter) {
			return msg.Deadletter(ctx)
		}
		return msg.Nack(ctx)
	}

	c.obs.observeSuccess(ctx, msg.Job(), start)

	ack := flow.RetryDelay(func(ctx context.Context) (struct{}, error) {
		return struct{}{}, msg.Ack(ctx)
	}, 3, c.opts.AckBackoff)
	_, err = ack(ctx)

	return err
}

// heartbeat periodically tells the driver a job is still being worked on,
// until ctx is cancelled (the handler returned, one way or another).
func (c *Consumer) heartbeat(ctx context.Context, pr ProgressReporter) {
	ticker := time.NewTicker(c.opts.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = pr.InProgress(ctx)
		}
	}
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
