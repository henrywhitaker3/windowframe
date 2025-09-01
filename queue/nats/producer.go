package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/henrywhitaker3/windowframe/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ProducerOpts struct {
	URL           string
	ConnectOpts   []nats.Option
	JetStreamOpts []jetstream.JetStreamOpt
}

type Producer struct {
	js jetstream.JetStream
}

func NewProducer(opts ProducerOpts) (*Producer, error) {
	conn, err := nats.Connect(opts.URL, opts.ConnectOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := jetstream.New(conn, opts.JetStreamOpts...)
	if err != nil {
		return nil, fmt.Errorf("create js object: %w", err)
	}

	return &Producer{
		js: js,
	}, nil
}

const (
	DelayedHeader = "Nak-Until"
)

func (p *Producer) Push(
	ctx context.Context,
	task queue.Task,
	payload []byte,
	opts ...queue.Option,
) error {
	msg := nats.NewMsg(fmt.Sprintf("%s.%s", string(queue.QueueFromOptions(opts)), string(task)))
	msg.Data = payload

	var runAt time.Time
	for _, opt := range opts {
		switch opt.Type() {
		case queue.AfterOpt:
			runAt = time.Now().Add(opt.Value().(time.Duration))
		case queue.AtOpt:
			runAt = opt.Value().(time.Time)
		}
	}
	if !runAt.IsZero() {
		msg.Header.Add(DelayedHeader, runAt.UTC().Format(time.RFC3339))
	}

	_, err := p.js.PublishMsg(ctx, msg, jsOptsFromQueueOpts(opts)...)
	return err
}

func (p *Producer) Close(ctx context.Context) error {
	p.js.Conn().Close()
	return nil
}

func jsOptsFromQueueOpts(opts []queue.Option) []jetstream.PublishOpt {
	out := []jetstream.PublishOpt{}
	hasId := false
	for _, opt := range opts {
		switch opt.Type() {
		case queue.IDOpt:
			out = append(out, jetstream.WithMsgID(opt.Value().(string)))
			hasId = true
		case queue.AfterOpt:
			// Handled in the Push method with headers
		case queue.AtOpt:
			// Handled in the Push method with headers
		case queue.MaxTriesOpt:
			// Not handled as it is set in the stream config
		}
	}
	if !hasId {
		out = append(out, jetstream.WithMsgID(uuid.MustNew().String()))
	}
	return out
}
