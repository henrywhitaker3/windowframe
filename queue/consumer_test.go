package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

type fakeConsumer struct {
	jobs        chan job
	hits        int
	deadletters int
	skips       int
	errors      int
}

func (f *fakeConsumer) add(job job) {
	f.jobs <- job
}

func (f *fakeConsumer) Consume(ctx context.Context, h HandlerFunc) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-f.jobs:
			by, _ := json.Marshal(job)
			err := h(ctx, by)
			f.hits++
			if err == nil {
				continue
			}
			f.errors++
			if errors.Is(err, ErrDeadLetter) {
				f.deadletters++
			}
			if errors.Is(err, ErrSkipRetry) {
				f.skips++
			}
		}
	}

}

func (f *fakeConsumer) Close(context.Context) error {
	return nil
}

const (
	FakeTask          Task = "fake"
	FakeTaskNoHandler Task = "fake_no_handler"
)

func TestItConsumesJobs(t *testing.T) {
	fake := &fakeConsumer{
		jobs: make(chan job, 100),
	}
	c := NewConsumer(ConsumerOpts{
		Consumer: fake,
		Observer: NewObserver(ObserverOpts{
			Logger: test.NewLogger(t),
		}),
	})

	c.RegisterHandler(FakeTask, func(ctx context.Context, payload []byte) error {
		t.Log(payload)
		var str string
		require.Nil(t, json.Unmarshal(payload, &str))
		if str == "bongo" {
			return nil
		}
		return fmt.Errorf("something went wrong")
	})

	job, err := newJob(FakeTask, "bongo")
	require.Nil(t, err)
	jobWithErr, err := newJob(FakeTask, "bongos")
	require.Nil(t, err)
	jobNoH, err := newJob(FakeTaskNoHandler, "bongo")
	require.Nil(t, err)
	fake.add(job)
	fake.add(jobNoH)
	fake.add(jobWithErr)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	go c.Consume(ctx)

	time.Sleep(time.Second)

	require.Equal(t, 3, fake.hits)
	require.Equal(t, 2, fake.errors)
	require.Equal(t, 1, fake.deadletters)
}
