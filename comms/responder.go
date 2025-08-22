package comms

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Responder struct {
	conn *nats.Conn
}

type ResponderOpts struct {
	URL         string
	ConnectOpts []nats.Option
}

func NewResponder(opts ResponderOpts) (*Responder, error) {
	conn, err := nats.Connect(opts.URL, opts.ConnectOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	return &Responder{
		conn: conn,
	}, nil
}

type response struct {
	Data  []byte `json:"data"`
	Error string `json:"error"`
}

func (r *Responder) Listen(
	ctx context.Context,
	topic Topic,
	h func(context.Context, []byte) ([]byte, error),
) error {
	sub, err := r.conn.Subscribe(string(topic), func(msg *nats.Msg) {
		resp, err := h(ctx, msg.Data)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		by, _ := json.Marshal(response{
			Data:  resp,
			Error: errStr,
		})
		_ = msg.Respond(by)
	})
	if err != nil {
		return fmt.Errorf("subscribe to subject: %w", err)
	}
	<-ctx.Done()
	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("unsubscribe from stream: %w", err)
	}
	return nil
}
