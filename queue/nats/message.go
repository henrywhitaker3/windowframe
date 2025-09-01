package nats

import (
	"context"
	"fmt"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/nats-io/nats.go/jetstream"
)

type message struct {
	job queue.Job
	msg jetstream.Msg
	kv  jetstream.KeyValue
}

func newMessage(job queue.Job, msg jetstream.Msg, kv jetstream.KeyValue) *message {
	return &message{
		job: job,
		msg: msg,
		kv:  kv,
	}
}

func (m *message) Job() queue.Job {
	return m.job
}

func (m *message) Ack(ctx context.Context) error {
	if m.kv != nil {
		if _, err := m.kv.Create(ctx, m.job.ID, []byte("processed")); err != nil {
			return fmt.Errorf("log ack: %w", err)
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

func (m *message) Nack(ctx context.Context) error {
	return m.msg.Nak()
}
