package events

import (
	"context"
	"errors"
)

var (
	ErrInvalidEventType = errors.New("invalid event type for listener")
)

// wrapListener gets around go's generic limitations to have type-safe listeners
// for events without having to .(Event) in the listener.
func wrapListeners[Event any](lis ...Listener[Event]) []Listener[any] {
	out := []Listener[any]{}
	for _, l := range lis {
		out = append(out, func(ctx context.Context, a any) error {
			val, ok := a.(Event)
			if !ok {
				return ErrInvalidEventType
			}
			return l(ctx, val)
		})
	}
	return out
}
