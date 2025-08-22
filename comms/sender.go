// Package comms
//
// Enabled two-way communication for separate processes via nats/redis/etc
package comms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type Topic string

type Sender struct {
	conn    *nats.Conn
	timeout time.Duration
}

type SenderOpts struct {
	URL         string
	ConnectOpts []nats.Option
	// How long to wait for a response from the responder (default: 3s)
	Timeout time.Duration
}

func NewSender(opts SenderOpts) (*Sender, error) {
	conn, err := nats.Connect(opts.URL, opts.ConnectOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}
	if opts.Timeout == 0 {
		opts.Timeout = time.Second * 3
	}
	return &Sender{
		conn:    conn,
		timeout: opts.Timeout,
	}, nil
}

func (s *Sender) Send(ctx context.Context, topic Topic, payload []byte) ([]byte, error) {
	reply, err := s.conn.Request(string(topic), payload, s.timeout)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	var response response
	if err := json.Unmarshal(reply.Data, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	var erro error
	if response.Error != "" {
		erro = fmt.Errorf("%v", response.Error)
	}

	return response.Data, erro
}
