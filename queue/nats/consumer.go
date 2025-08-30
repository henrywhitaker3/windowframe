// Package nats
package nats

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ConsumerOpts struct {
	URL           string
	StreamName    string
	ConnectOpts   []nats.Option
	JetStreamOpts []jetstream.JetStreamOpt

	Concurrency int
}

type Consumer struct {
	js jetstream.JetStream

	opts ConsumerOpts
}

func NewConsumer(opts ConsumerOpts) (*Consumer, error) {
	conn, err := nats.Connect(opts.URL, opts.ConnectOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := jetstream.New(conn, opts.JetStreamOpts...)
	if err != nil {
		return nil, fmt.Errorf("create js instance: %w", err)
	}

	if opts.Concurrency == 0 {
		opts.Concurrency = runtime.NumCPU()
	}

	return &Consumer{
		js:   js,
		opts: opts,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, h queue.HandlerFunc) error {
	stream, err := c.js.Stream(ctx, c.opts.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name: c.opts.StreamName,
	})
	if err != nil {
		return fmt.Errorf("create or update consumer: %w", err)
	}

	pool := make(chan struct{}, c.opts.Concurrency)
	defer close(pool)

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			batch, err := cons.Fetch(c.opts.Concurrency, jetstream.FetchMaxWait(time.Second))
			if err != nil {
				time.Sleep(time.Millisecond * 50)
				continue
			}
			for msg := range batch.Messages() {
				wg.Go(func() {
					pool <- struct{}{}
					c.handleMessage(ctx, msg, h)
					<-pool
				})
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg jetstream.Msg, h queue.HandlerFunc) {
	if untilRaw := msg.Headers().Get(DelayedHeader); untilRaw != "" {
		t, _ := time.Parse(time.RFC3339, untilRaw)
		until := time.Until(t)
		if until > 0 {
			_ = msg.NakWithDelay(until)
			return
		}
	}
	if err := h(ctx, msg.Data()); err != nil {
		if errors.Is(err, queue.ErrSkipRetry) {
			_ = msg.Term()
			return
		}
		if errors.Is(err, queue.ErrDeadLetter) {
			// TODO: add deadletter functionality
			_ = msg.TermWithReason("deadletter")
			return
			// panic("deadletter not impleneted in nats consumers")
		}
		_ = msg.Nak()
	}
	_ = msg.Ack()
}

func (c *Consumer) Close(ctx context.Context) error {
	c.js.Conn().Close()
	return nil
}
