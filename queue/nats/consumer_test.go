package nats

import (
	"context"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/v2/queue"
	"github.com/henrywhitaker3/windowframe/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

var _ jetstream.Msg = (*fakeMsg)(nil)

// fakeMsg lets tests drive Consumer.handleMessage directly with synthetic
// deliveries (e.g. simulating a redelivery) without depending on real NATS
// AckWait/redelivery timing.
type fakeMsg struct {
	headers nats.Header
	data    []byte

	doubleAcked  bool
	nakked       bool
	nakDelay     time.Duration
	termed       bool
	termReason   string
	inProgressed bool
}

func newFakeMsg(id string, job queue.Job) *fakeMsg {
	h := nats.Header{}
	h.Set("Nats-Msg-Id", id)
	by, err := queue.Marshal(job)
	if err != nil {
		panic(err)
	}
	return &fakeMsg{headers: h, data: by}
}

func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return &jetstream.MsgMetadata{}, nil }
func (m *fakeMsg) Data() []byte                              { return m.data }
func (m *fakeMsg) Headers() nats.Header                      { return m.headers }
func (m *fakeMsg) Subject() string                           { return "test.subject" }
func (m *fakeMsg) Reply() string                             { return "" }
func (m *fakeMsg) Ack() error                                { return nil }
func (m *fakeMsg) DoubleAck(ctx context.Context) error       { m.doubleAcked = true; return nil }
func (m *fakeMsg) Nak() error                                { m.nakked = true; return nil }
func (m *fakeMsg) NakWithDelay(d time.Duration) error {
	m.nakked = true
	m.nakDelay = d
	return nil
}
func (m *fakeMsg) InProgress() error { m.inProgressed = true; return nil }
func (m *fakeMsg) Term() error       { m.termed = true; return nil }
func (m *fakeMsg) TermWithReason(reason string) error {
	m.termed = true
	m.termReason = reason
	return nil
}

// setupClaimTestConsumer boots a real NATS/JetStream consumer (KV claims are
// exercised for real) but lets the test feed it synthetic deliveries via
// handleMessage instead of relying on real redelivery timing.
func setupClaimTestConsumer(t *testing.T) (*Consumer, chan queue.Message) {
	t.Helper()

	natsURL, cancel := test.Nats(t)
	t.Cleanup(cancel)

	test.NatsStream(t, natsURL, jetstream.StreamConfig{
		Name:      "dedupe",
		Subjects:  []string{"dedupe.>"},
		Retention: jetstream.WorkQueuePolicy,
	})

	c, err := NewConsumer(ConsumerOpts{
		URL:                  natsURL,
		StreamName:           "dedupe",
		ProcessedLogReplicas: 1,
	})
	require.Nil(t, err)

	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	out := make(chan queue.Message, 10)
	require.Nil(t, c.Consume(ctx, out))
	t.Cleanup(func() { _ = c.Close(context.Background()) })

	return c, out
}

// TestProcessedLogRedeliveryWhileInFlight covers the scenario a prior review
// flagged: a message gets redelivered (e.g. AckWait firing) while the first
// delivery is still being processed. It must be nak'd for later retry, not
// permanently terminated - terminating it would lose the job for good if the
// original attempt goes on to fail. Once the original attempt genuinely
// completes, a further redelivery IS safe to terminate.
func TestProcessedLogRedeliveryWhileInFlight(t *testing.T) {
	c, out := setupClaimTestConsumer(t)
	ctx := context.Background()

	job := queue.NewJob("job-in-flight", queue.Task("demo"), []byte("payload"))

	first := newFakeMsg("job-in-flight", job)
	c.handleMessage(ctx, first, out)
	require.False(t, first.termed)
	require.False(t, first.nakked)
	require.Len(t, out, 1)

	entry, err := c.kv.Get(ctx, "job-in-flight")
	require.Nil(t, err)
	require.Equal(t, kvStateProcessing, string(entry.Value()))

	// A redelivery arrives while the first attempt is still in flight.
	redelivery := newFakeMsg("job-in-flight", job)
	c.handleMessage(ctx, redelivery, out)
	require.True(t, redelivery.nakked, "in-flight redelivery should be nak'd for later retry")
	require.False(t, redelivery.termed, "in-flight redelivery must not be permanently terminated")
	require.Len(
		t,
		out,
		1,
		"in-flight redelivery must not be forwarded for processing a second time",
	)

	// The original attempt completes successfully.
	msg := <-out
	require.Nil(t, msg.Ack(ctx))
	require.True(t, first.doubleAcked)

	entry, err = c.kv.Get(ctx, "job-in-flight")
	require.Nil(t, err)
	require.Equal(t, kvStateProcessed, string(entry.Value()))

	// Now that it's genuinely complete, a further redelivery is safe to
	// terminate permanently.
	late := newFakeMsg("job-in-flight", job)
	c.handleMessage(ctx, late, out)
	require.True(t, late.termed, "redelivery of a completed message should be terminated")
	require.Len(t, out, 0, "completed message must not be forwarded again")
}

// TestProcessedLogStaleClaimAfterCrash covers a worker that claims a message
// and then crashes before ever acking or nacking it. The stale "processing"
// claim it leaves behind must not be treated as a completed message - the
// job would then be silently lost. It should be nak'd so it can eventually
// be retried once (in real usage) the claim's TTL expires or the process
// wakes back up and nacks it.
func TestProcessedLogStaleClaimAfterCrash(t *testing.T) {
	c, out := setupClaimTestConsumer(t)
	ctx := context.Background()

	job := queue.NewJob("job-stale", queue.Task("demo"), []byte("payload"))

	crashed := newFakeMsg("job-stale", job)
	c.handleMessage(ctx, crashed, out)
	require.Len(t, out, 1)
	<-out // the worker picks this up, then crashes without acking or nacking

	redelivery := newFakeMsg("job-stale", job)
	c.handleMessage(ctx, redelivery, out)
	require.True(t, redelivery.nakked, "a stale claim must be nak'd, not terminated")
	require.False(t, redelivery.termed, "a stale claim must not permanently block retry")
	require.Len(t, out, 0)
}

// TestProcessedLogNackReleasesClaim ensures a handler failure (Nack) frees
// the claim immediately, so the very next delivery can be reprocessed rather
// than waiting out the claim's TTL.
func TestProcessedLogNackReleasesClaim(t *testing.T) {
	c, out := setupClaimTestConsumer(t)
	ctx := context.Background()

	job := queue.NewJob("job-nacked", queue.Task("demo"), []byte("payload"))

	first := newFakeMsg("job-nacked", job)
	c.handleMessage(ctx, first, out)
	require.Len(t, out, 1)

	msg := <-out
	require.Nil(t, msg.Nack(ctx))
	require.True(t, first.nakked)

	_, err := c.kv.Get(ctx, "job-nacked")
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound, "nack should release the claim")

	retry := newFakeMsg("job-nacked", job)
	c.handleMessage(ctx, retry, out)
	require.False(t, retry.nakked)
	require.False(t, retry.termed)
	require.Len(t, out, 1, "a released claim should allow reprocessing")
}
