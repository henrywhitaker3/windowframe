package nats

import (
	"context"
	"fmt"

	"github.com/henrywhitaker3/windowframe/v2/queue"
	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/nats-io/nats.go/jetstream"
)

type message struct {
	job queue.Job
	msg jetstream.Msg
	kv  jetstream.KeyValue
	// The Nats-Msg-Id header value the consumer claimed in the processed log.
	id string
}

func newMessage(job queue.Job, msg jetstream.Msg, kv jetstream.KeyValue, id string) *message {
	return &message{
		job: job,
		msg: msg,
		kv:  kv,
		id:  id,
	}
}

func (m *message) Job() queue.Job {
	return m.job
}

// Ack marks the processed-log claim as complete, then acknowledges the
// message. Marking completion is what lets the consumer safely Term a
// redelivery of this id later instead of merely nak-ing it.
func (m *message) Ack(ctx context.Context) error {
	if m.kv != nil {
		ctx, span := tracing.NewSpan(ctx, "MarkProcessedLog")
		defer span.End()
		if _, err := m.kv.Put(ctx, m.id, []byte(kvStateProcessed)); err != nil {
			return fmt.Errorf("mark processed: %w", err)
		}
	}
	return m.msg.DoubleAck(ctx)
}

func (m *message) Cancel(ctx context.Context) error {
	return m.msg.Term()
}

func (m *message) Deadletter(ctx context.Context) error {
	return m.msg.TermWithReason("deadlettered")
}

// Nack releases the processed-log claim so the redelivered message is
// allowed to be processed again, then puts the message back onto the queue.
func (m *message) Nack(ctx context.Context) error {
	if m.kv != nil {
		ctx, span := tracing.NewSpan(ctx, "ReleaseProcessedLogClaim")
		defer span.End()
		_ = m.kv.Delete(ctx, m.id)
	}
	return m.msg.Nak()
}

// InProgress signals to the server that this message is still being worked
// on, resetting the ack-wait timer so a slow-but-healthy handler isn't
// redelivered mid-processing.
func (m *message) InProgress(ctx context.Context) error {
	return m.msg.InProgress()
}
