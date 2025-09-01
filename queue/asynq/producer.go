// Package asynq
package asynq

import (
	"context"
	"fmt"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/hibiken/asynq"
)

type Producer struct {
	client *asynq.Client
}

type ProducerOpts struct {
	Redis RedisOpts
}

func NewProducer(opts ProducerOpts) (*Producer, error) {
	client := asynq.NewClientFromRedisClient(opts.Redis.Client())
	if err := client.Ping(); err != nil {
		return nil, err
	}

	return &Producer{client: client}, nil
}

func (p *Producer) Push(
	ctx context.Context,
	job queue.Job,
) error {
	by, err := queue.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	task := asynq.NewTask(string(job.Task), by, asynqOptsFromQueueOpts(job.Options)...)
	_, err = p.client.EnqueueContext(
		ctx,
		task,
		asynq.Queue(string(queue.QueueFromOptions(job.Options))),
	)
	return err
}

func (p *Producer) Close(ctx context.Context) error {
	return p.client.Close()
}

func asynqOptsFromQueueOpts(opts []queue.Option) []asynq.Option {
	out := []asynq.Option{}
	for _, opt := range opts {
		switch opt.Type() {
		case queue.AtOpt:
			out = append(out, asynq.ProcessAt(opt.Value().(time.Time)))
		case queue.AfterOpt:
			out = append(out, asynq.ProcessIn(opt.Value().(time.Duration)))
		case queue.IDOpt:
			out = append(out, asynq.TaskID(opt.Value().(string)))
		case queue.MaxTriesOpt:
			out = append(out, asynq.MaxRetry(opt.Value().(int)))
		}
	}
	return out
}
