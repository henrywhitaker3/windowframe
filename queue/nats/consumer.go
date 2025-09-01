// Package nats
package nats

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/henrywhitaker3/flow"
	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ConsumerOpts struct {
	URL           string
	StreamName    string
	ConnectOpts   []nats.Option
	JetStreamOpts []jetstream.JetStreamOpt

	// How many messages are handled concurrently. Defaults to NumCPU.
	Concurrency int

	// Defaults to [500ms, 1s, 3s]
	Backoff []time.Duration
	// Defaults to 25
	MaxDelivery int

	// The number of messages to fetch at a time (default: 100)
	BatchSize int

	// How long to wait when fetching messages (default: 1s)
	FetchMaxWait time.Duration

	// How many times to attempt message ack (default: 3)
	AckAttempts int

	// How long to keep message processed data in KV store (default: 1h)
	ProcessedLogTTL time.Duration

	// How many replicas of the processed log to store (default: 3)
	ProcessedLogReplicas int
}

func (c ConsumerOpts) withDefaults() ConsumerOpts {
	if c.Concurrency == 0 {
		c.Concurrency = runtime.NumCPU()
	}
	if len(c.Backoff) == 0 {
		c.Backoff = []time.Duration{
			time.Millisecond * 500,
			time.Second,
			time.Second * 3,
		}
	}
	if c.MaxDelivery == 0 {
		c.MaxDelivery = 25
	}
	if c.BatchSize == 0 {
		c.BatchSize = 100
	}
	if c.FetchMaxWait == 0 {
		c.FetchMaxWait = time.Second
	}
	if c.AckAttempts == 0 {
		c.AckAttempts = 3
	}
	if c.ProcessedLogTTL == 0 {
		c.ProcessedLogTTL = time.Hour
	}
	if c.ProcessedLogReplicas == 0 {
		c.ProcessedLogReplicas = 3
	}
	return c
}

type Consumer struct {
	js jetstream.JetStream
	kv jetstream.KeyValue

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

	opts = opts.withDefaults()

	return &Consumer{
		js:   js,
		opts: opts,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, h queue.HandlerFunc) error {
	kv, err := c.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      c.opts.StreamName,
		Description: "The processed store log for the messages",
		TTL:         c.opts.ProcessedLogTTL,
		Replicas:    c.opts.ProcessedLogReplicas,
	})
	if err != nil {
		return fmt.Errorf("create processed log store: %w", err)
	}
	c.kv = kv

	stream, err := c.js.Stream(ctx, c.opts.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:       c.opts.StreamName,
		MaxDeliver: c.opts.MaxDelivery,
		BackOff:    c.opts.Backoff,
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
			batch, err := cons.Fetch(c.opts.BatchSize, jetstream.FetchMaxWait(c.opts.FetchMaxWait))
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
	if c.kv == nil {
		panic("kv processed log is nil")
	}

	id := msg.Headers().Get("Nats-Msg-Id")
	if id == "" {
		_ = msg.TermWithReason("message has no id")
		return
	}

	// If the key exists, then the message has already been processed
	if _, err := c.kv.Get(ctx, id); err == nil {
		_ = msg.TermWithReason("message already processed")
		return
	}

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

	ack := flow.Retry(func(ctx context.Context) (struct{}, error) {
		return struct{}{}, msg.DoubleAck(ctx)
	}, c.opts.AckAttempts)

	if _, err := ack(ctx); err == nil {
		store := flow.Retry(func(ctx context.Context) (struct{}, error) {
			_, err := c.kv.Create(ctx, id, []byte("processed"))
			return struct{}{}, err
		}, c.opts.AckAttempts)
		_, _ = store(ctx)
	}
}

func (c *Consumer) Close(ctx context.Context) error {
	c.js.Conn().Close()
	return nil
}
