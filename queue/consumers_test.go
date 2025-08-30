package queue_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/henrywhitaker3/windowframe/queue/asynq"
	"github.com/henrywhitaker3/windowframe/queue/nats"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

const (
	DemoTask  queue.Task  = "demo"
	DemoQueue queue.Queue = "demo"
)

func TestItProducesAndConsumesJobs(t *testing.T) {
	redis, cancel := test.Redis(t)
	defer cancel()

	natsURL, cancel := test.Nats(t)
	defer cancel()

	test.NatsStream(t, natsURL, jetstream.StreamConfig{
		Name:      "demo",
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{"demo.>"},
	})

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
				prod, err := asynq.NewProducer(asynq.ProducerOpts{
					Redis: asynq.RedisOpts{
						Addr: fmt.Sprintf("127.0.0.1:%d", redis),
					},
				})
				require.Nil(t, err)
				return prod
			},
		},
		{
			name: "nats",
			consumer: func(t *testing.T) queue.QueueConsumer {
				cons, err := nats.NewConsumer(nats.ConsumerOpts{
					URL:        natsURL,
					StreamName: "demo",
				})
				require.Nil(t, err)
				return cons
			},
			producer: func(t *testing.T) queue.QueueProducer {
				prod, err := nats.NewProducer(nats.ProducerOpts{
					URL: natsURL,
				})
				require.Nil(t, err)
				return prod
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			obs := queue.NewConsumerObserver(queue.ConsumerObserverOpts{
				Logger: test.NewLogger(t),
				Reg:    reg,
			})
			c := queue.NewConsumer(queue.ConsumerOpts{
				Consumer: tc.consumer(t),
				Observer: obs,
			})
			p := queue.NewProducer(queue.ProducerOpts{
				Producer: tc.producer(t),
				Observer: queue.NewProducerObserver(queue.ProducerObserverOpts{
					Logger: test.NewLogger(t),
					Reg:    reg,
				}),
			})

			handler := &fakeHandler{mu: &sync.Mutex{}, t: t}

			c.RegisterHandler(DemoTask, handler.Handler)

			assertProcessed(t, obs, 0)

			// No hits as no handler registered
			require.Nil(
				t,
				p.Push(
					ctx,
					queue.Task("bongo"),
					"bongo-no-handler",
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

			// Should be put in deadletter queue
			require.Nil(t, p.Push(ctx, DemoTask, "dead", queue.OnQueue(DemoQueue)))
			// Should error and be skipped
			require.Nil(t, p.Push(ctx, DemoTask, "skip", queue.OnQueue(DemoQueue)))

			go func() {
				if err := c.Consume(ctx); err != nil {
					panic(err)
				}
			}()
			defer c.Close(ctx)

			t.Log("waiting for initial processing")
			time.Sleep(time.Second * 5)

			assertProcessed(t, obs, 3)
			assertDeadlettered(t, obs, 1)
			assertSkipped(t, obs, 1)

			// Should register a hit after another second wait
			require.Nil(
				t,
				p.Push(
					ctx,
					DemoTask,
					"bongo-after-duration",
					queue.OnQueue(DemoQueue),
					queue.After(time.Second*10),
					queue.WithID("task-after-duration"),
				),
			)
			// Should register a hit after another second wait
			require.Nil(
				t,
				p.Push(
					ctx,
					DemoTask,
					"bongo-at-time",
					queue.OnQueue(DemoQueue),
					queue.WithID("task-at-time"),
					queue.At(time.Now().Add(time.Second*10)),
				),
			)
			t.Log("waiting for queued/delayed jobs to be processed")
			time.Sleep(time.Second * 12)
			assertProcessed(t, obs, 5)
		})
	}
}

func assertProcessed(t *testing.T, obs *queue.ConsumerObserver, value float64) {
	processed := test.GetCounterValue(t, obs.JobsProcessed.WithLabelValues(string(DemoTask)))
	require.Equal(t, value, processed)
}

func assertDeadlettered(t *testing.T, obs *queue.ConsumerObserver, value float64) {
	processed := test.GetCounterValue(t, obs.JobsDeadlettered.WithLabelValues(string(DemoTask)))
	require.Equal(t, value, processed)
}

func assertSkipped(t *testing.T, obs *queue.ConsumerObserver, value float64) {
	processed := test.GetCounterValue(t, obs.JobsSkipped.WithLabelValues(string(DemoTask)))
	require.Equal(t, value, processed)
}

type fakeHandler struct {
	t    *testing.T
	mu   *sync.Mutex
	hits int
}

func (f *fakeHandler) Handler(ctx context.Context, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.hits++

	var str string
	if err := queue.Unmarshal(payload, &str); err != nil {
		return err
	}

	f.t.Log("got payload", str)

	if strings.Contains(str, "bongo") {
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
