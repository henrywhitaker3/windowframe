// Package nats
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/henrywhitaker3/windowframe/v2/queue"
	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Values stored against a message id in the processed-log KV store.
const (
	// The id has been claimed for processing but not yet confirmed complete -
	// this could mean it's genuinely in flight, or that the attempt which
	// claimed it crashed/never finished.
	kvStateProcessing = "processing"
	// The id was successfully processed and acked. Safe to permanently
	// terminate redelivery.
	kvStateProcessed = "processed"
)

type ConsumerOpts struct {
	URL           string
	StreamName    string
	ConnectOpts   []nats.Option
	JetStreamOpts []jetstream.JetStreamOpt

	// Defaults to [500ms, 1s, 3s]
	Backoff []time.Duration
	// Defaults to 25
	MaxDelivery int
	// How long the server waits for an ack before redelivering a message.
	// Should comfortably exceed HeartbeatInterval on the consumer so a
	// slow-but-alive handler isn't redelivered mid-processing (default: 1m)
	AckWait time.Duration

	// The number of messages to fetch at a time (default: 100)
	BatchSize int

	// How long to wait when fetching messages (default: 1s)
	FetchMaxWait time.Duration

	// Whether to enabled the JetStream KV process log for extra protection
	// against duplicate message processing (default: true)
	ProcessLogEnabled *bool

	// How long to keep message processed data in KV store (default: 1h)
	ProcessedLogTTL time.Duration

	// How many replicas of the processed log to store (default: 3)
	ProcessedLogReplicas int
}

func (c ConsumerOpts) withDefaults() ConsumerOpts {
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
	if c.AckWait == 0 {
		c.AckWait = time.Minute
	}
	if c.BatchSize == 0 {
		c.BatchSize = 100
	}
	if c.FetchMaxWait == 0 {
		c.FetchMaxWait = time.Second
	}
	if c.ProcessLogEnabled == nil {
		c.ProcessLogEnabled = Ptr(true)
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

func (c *Consumer) Consume(
	ctx context.Context,
	out chan<- queue.Message,
) error {
	stream, err := c.js.Stream(ctx, c.opts.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	if *c.opts.ProcessLogEnabled {
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
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:       c.opts.StreamName,
		MaxDeliver: c.opts.MaxDelivery,
		BackOff:    c.opts.Backoff,
		AckWait:    c.opts.AckWait,
	})
	if err != nil {
		return fmt.Errorf("create or update consumer: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				batch, err := cons.Fetch(
					c.opts.BatchSize,
					jetstream.FetchMaxWait(c.opts.FetchMaxWait),
				)
				if err != nil {
					time.Sleep(time.Millisecond * 50)
					continue
				}
				for msg := range batch.Messages() {
					c.handleMessage(ctx, msg, out)
				}
			}
		}
	}()

	return nil
}

func (c *Consumer) handleMessage(ctx context.Context, msg jetstream.Msg, out chan<- queue.Message) {
	if *c.opts.ProcessLogEnabled && c.kv == nil {
		panic("kv processed log is nil")
	}

	id := msg.Headers().Get("Nats-Msg-Id")
	if id == "" {
		_ = msg.TermWithReason("message has no id")
		return
	}

	// Delayed messages get nak'd-with-delay and redelivered once due, so this
	// must happen before the processed-log claim below - otherwise the first
	// (premature) delivery would permanently claim the id and the message
	// would be dropped as "already processed" once it actually comes due.
	if untilRaw := msg.Headers().Get(DelayedHeader); untilRaw != "" {
		t, _ := time.Parse(time.RFC3339, untilRaw)
		until := time.Until(t)
		if until > 0 {
			_ = msg.NakWithDelay(until)
			return
		}
	}

	if *c.opts.ProcessLogEnabled {
		ctx, span := tracing.NewSpan(ctx, "ClaimProcessedLog")
		defer span.End()
		// Atomically claim the message id. This fails if the id is already
		// claimed (in flight or completed), which closes the race where a
		// redelivered message could pass a plain existence check before the
		// original delivery has finished processing and acked.
		if _, err := c.kv.Create(ctx, id, []byte(kvStateProcessing)); err != nil {
			if errors.Is(err, jetstream.ErrKeyExists) {
				// The id is already claimed, but that alone doesn't tell us
				// whether it finished successfully, is still being worked on,
				// or was orphaned by a crash. Only Term - which permanently
				// stops redelivery - once we can confirm via the KV value
				// that it actually completed. Otherwise nak so the job isn't
				// lost forever if the original attempt later fails or never
				// finishes.
				entry, getErr := c.kv.Get(ctx, id)
				if getErr == nil && string(entry.Value()) == kvStateProcessed {
					_ = msg.TermWithReason("message already processed")
					return
				}
				_ = msg.Nak()
				return
			}
			// Unable to verify the claim - nak and let it be retried rather
			// than risk processing a message we couldn't dedupe.
			_ = msg.Nak()
			return
		}
		span.End()
	}
	var job queue.Job
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		_ = msg.Nak()
		return
	}
	out <- newMessage(job, msg, c.kv, id)
}

func (c *Consumer) Close(ctx context.Context) error {
	c.js.Conn().Close()
	return nil
}

func Ptr[T any](v T) *T {
	return &v
}
