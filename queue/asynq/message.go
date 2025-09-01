package asynq

import (
	"context"
	"fmt"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/hibiken/asynq"
)

type message struct {
	job queue.Job
	out chan error
}

func newMessage(job queue.Job, out chan error) *message {
	return &message{
		job: job,
		out: out,
	}
}

func (m *message) Job() queue.Job {
	return m.job
}

func (m *message) Ack(ctx context.Context) error {
	m.out <- nil
	return nil
}

func (m *message) Cancel(ctx context.Context) error {
	m.out <- asynq.RevokeTask
	return nil
}

func (m *message) Deadletter(ctx context.Context) error {
	m.out <- asynq.SkipRetry
	return nil
}

func (m *message) Nack(ctx context.Context) error {
	m.out <- fmt.Errorf("task nacked")
	return nil
}
