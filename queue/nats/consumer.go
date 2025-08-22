// Package nats
package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
}

type Consumer struct {
	js     jetstream.JetStream
	stream string
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

	return &Consumer{
		js:     js,
		stream: opts.StreamName,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, h queue.HandlerFunc) error {
	stream, err := c.js.Stream(ctx, c.stream)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name: strings.ReplaceAll(strings.ReplaceAll(c.stream, ".", "-"), ">", "-"),
	})
	if err != nil {
		return fmt.Errorf("create or update consumer: %w", err)
	}

	cctx, err := cons.Consume(func(msg jetstream.Msg) {
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
	})
	if err != nil {
		return fmt.Errorf("consume message from consumer: %w", err)
	}

	<-cctx.Closed()
	return nil
}

func (c *Consumer) Close(ctx context.Context) error {
	c.js.Conn().Close()
	return nil
}
