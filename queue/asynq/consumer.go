package asynq

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type Consumer struct {
	server *asynq.Server
}

type ConsumerOpts struct {
	Queues []queue.Queue
	Redis  RedisOpts
	// The number of concurrent jobs the worker processes (default: num cpu)
	Concurrency int

	Logger *slog.Logger
}

type RedisOpts struct {
	Addr        string
	Password    string
	DB          int
	OtelEnabled bool
}

func (r RedisOpts) Client() redis.UniversalClient {
	client := redis.NewClient(&redis.Options{
		Addr:     r.Addr,
		Password: r.Password,
		DB:       r.DB,
	})

	if r.OtelEnabled {
		_ = redisotel.InstrumentTracing(client, redisotel.WithDBStatement(true))
	}

	return client
}

func NewConsumer(ctx context.Context, opts ConsumerOpts) (*Consumer, error) {
	queues := map[string]int{}
	for _, queue := range opts.Queues {
		queues[string(queue)] = 9
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = runtime.NumCPU()
	}
	srv := asynq.NewServerFromRedisClient(
		opts.Redis.Client(),
		asynq.Config{
			Concurrency: opts.Concurrency,
			BaseContext: func() context.Context { return ctx },
			Queues:      queues,
			Logger: &asynqLogger{
				log: opts.Logger,
			},
		},
	)
	if err := srv.Ping(); err != nil {
		return nil, err
	}

	return &Consumer{
		server: srv,
	}, nil
}

func (w *Consumer) Close(ctx context.Context) error {
	w.server.Shutdown()
	return nil
}

func (w *Consumer) Consume(ctx context.Context) (<-chan queue.Message, error) {
	out := make(chan queue.Message, 1000)
	startErr := make(chan error)
	go func() {
		err := w.server.Start(asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			var job queue.Job
			if err := queue.Unmarshal(t.Payload(), &job); err != nil {
				return fmt.Errorf("unmarshal job: %w", err)
			}
			err := make(chan error, 1)
			msg := newMessage(job, err)
			out <- msg
			return <-err
		}))
		startErr <- err
	}()
	return out, <-startErr
}
