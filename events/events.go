// Package events
package events

import (
	"context"
	"errors"
	"fmt"
)

type Subscriber[T any] interface {
	Handle(context.Context, T) error
}

type subscriber func() func(context.Context, []byte) error

type Broker interface {
	Send(context.Context, string, []byte) error
	Subscribe(context.Context, string) (<-chan []byte, error)
}

type Bus struct {
	broker      Broker
	subscribers map[string][]subscriber
	encoder     func(any) ([]byte, error)
	decoder     func([]byte, any) error
}

var (
	ErrFailedDecoding = errors.New("failed to decode event")
)

func Subscribe[T any](b *Bus, topic string, s Subscriber[T]) {
	b.subscribers[topic] = append(b.subscribers[topic], func() func(context.Context, []byte) error {
		return func(ctx context.Context, d []byte) error {
			var val T
			if err := b.decoder(d, &val); err != nil {
				return fmt.Errorf("%w: %w", ErrFailedDecoding, err)
			}
			return s.Handle(ctx, val)
		}
	})
}

var (
	ErrFailedEncoding = errors.New("failed to encode event")
	ErrFailedTransmit = errors.New("failed to transmit event")
)

func (b *Bus) Publish(ctx context.Context, topic string, msg any) error {
	data, err := b.encoder(msg)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedEncoding, err)
	}
	if err := b.broker.Send(ctx, topic, data); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedTransmit, err)
	}
	return nil
}
