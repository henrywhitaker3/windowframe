package queue_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/henrywhitaker3/windowframe/queue/asynq"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

const (
	DemoTask  queue.Task  = "demo"
	DemoQueue queue.Queue = "demo"
)

func TestItProducesAndConsumesJobs(t *testing.T) {
	redis, cancel := test.Redis(t)
	defer cancel()

	tcs := []struct {
		name         string
		consumer     func(*testing.T) queue.QueueConsumer
		producer     func(*testing.T) queue.QueueProducer
		pushed       int
		processed    int
		errored      int
		deadlettered int
	}{
		{
			name: "asynq",
			consumer: func(t *testing.T) queue.QueueConsumer {
				cons, err := asynq.NewConsumer(context.TODO(), asynq.ConsumerOpts{
					Queues: []queue.Queue{DemoQueue},
					Redis: asynq.RedisOpts{
						Addr: fmt.Sprintf("127.0.0.1:%d", redis),
					},
					Logger: test.NewLogger(t),
				})
				require.Nil(t, err)
				return cons
			},
			producer: func(t *testing.T) queue.QueueProducer {
				prod, err := asynq.NewPublisher(asynq.PublisherOpts{
					Redis: asynq.RedisOpts{
						Addr: fmt.Sprintf("127.0.0.1:%d", redis),
					},
				})
				require.Nil(t, err)
				return prod
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := queue.NewConsumer(queue.ConsumerOpts{
				Consumer: tc.consumer(t),
				Observer: queue.NewObserver(queue.ObserverOpts{
					Logger: test.NewLogger(t),
				}),
			})
			p := queue.NewProducer(queue.ProducerOpts{
				Producer: tc.producer(t),
				Observer: queue.NewObserver(queue.ObserverOpts{
					Logger: test.NewLogger(t),
				}),
			})

			handler := &fakeHandler{mu: &sync.Mutex{}}

			c.RegisterHandler(DemoTask, handler.Handler)

			// No hits as no handler registered
			require.Nil(
				t,
				p.Push(
					ctx,
					queue.Task("bongo"),
					"bongo",
					queue.OnQueue(DemoQueue),
					queue.WithID("no-handler"),
				),
			)
			// Should register a hit
			require.Nil(
				t,
				p.Push(
					ctx,
					DemoTask,
					"bongo",
					queue.OnQueue(DemoQueue),
					queue.WithID("standard-job"),
				),
			)
			// No hits as queue not being consumed
			require.Nil(t, p.Push(ctx, DemoTask, "bongo"), queue.WithID("no-queue"))
			// Should error as invalid string
			// require.Nil(t, p.Push(ctx, DemoTask, "bingo", queue.OnQueue(DemoQueue)))
			// Should be put in deadletter queue
			// require.Nil(t, p.Push(ctx, DemoTask, "dead", queue.OnQueue(DemoQueue)))
			// Should be skipped and not deadlettered
			// require.Nil(t, p.Push(ctx, DemoTask, "skip", queue.OnQueue(DemoQueue)))
			// Should register a hit after another second wait
			require.Nil(
				t,
				p.Push(
					ctx,
					DemoTask,
					"bongo",
					queue.OnQueue(DemoQueue),
					queue.After(time.Millisecond*1500),
					queue.WithID("task-after-duration"),
				),
			)
			// Should register a hit after another second wait
			require.Nil(
				t,
				p.Push(
					ctx,
					DemoTask,
					"bongo",
					queue.OnQueue(DemoQueue),
					queue.WithID("task-at-time"),
					queue.At(time.Now().Add(time.Second)),
				),
			)

			go c.Consume(ctx)
			defer c.Close(ctx)

			t.Log("waiting for initial processing")
			time.Sleep(time.Second)
			require.Equal(t, 1, handler.hits)
			t.Log("waiting for queued/delayed jobs to be processed")
			time.Sleep(time.Second)
			require.Equal(t, 3, handler.hits)
		})
	}
}

type fakeHandler struct {
	mu   *sync.Mutex
	hits int
}

func (f *fakeHandler) Handler(ctx context.Context, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.hits++

	var str string
	if err := json.Unmarshal(payload, &str); err != nil {
		return err
	}

	if str == "bongo" {
		return nil
	}
	if str == "dead" {
		return queue.ErrDeadLetter
	}
	if str == "skip" {
		return queue.ErrSkipRetry
	}

	return fmt.Errorf("not bongo")
}
