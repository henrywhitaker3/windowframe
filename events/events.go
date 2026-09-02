// Package events
package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"sync"
	"time"
)

var (
	global *EventHandler
)

// SetGlobal stores a single event handler in a private variable so the handler
// doesn't need to be a direct dependency everywhere that emits events.
//
// Use Dispatch() instead of Event() to use the global handler.
func SetGlobal(h *EventHandler) {
	global = h
}

type Listener[Event any] func(context.Context, Event) error

type EventHandler struct {
	slog *slog.Logger

	opts EventHandlerOptions

	wg      *sync.WaitGroup
	running *sync.WaitGroup
	events  chan func()
	closed  chan struct{}

	mu       *sync.RWMutex
	handlers map[string][]Listener[any]
}

type EventHandlerOptions struct {
	// The duration passed to context.WithTimeout for non-queued events
	HandlerTimeout time.Duration
}

func New(opts EventHandlerOptions) *EventHandler {
	return &EventHandler{
		slog:     slog.With("component", "events"),
		opts:     opts,
		wg:       &sync.WaitGroup{},
		running:  &sync.WaitGroup{},
		events:   make(chan func(), 100),
		closed:   make(chan struct{}),
		mu:       &sync.RWMutex{},
		handlers: map[string][]Listener[any]{},
	}
}

func (e *EventHandler) Listen[Event any](lis ...Listener[Event]) error {
	name, err := name[Event]()
	if err != nil {
		return fmt.Errorf("namespace event: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.slog.Debug("registering event", "event", name, "listeners", len(lis))

	e.handlers[name] = append(e.handlers[name], wrapListeners(lis...)...)

	return nil
}

func Listen[Event any](lis ...Listener[Event]) error {
	if global == nil {
		panic("cannot process event on nil global")
	}
	return global.Listen(lis...)
}

var (
	ErrUnregisteredEvent = errors.New("unregistered event")
)

func (e *EventHandler) Dispatch[Event any](event Event) error {
	handlers, err := e.getHandlers[Event]()
	if err != nil {
		return fmt.Errorf("namespace event: %w", err)
	}
	return e.dispatch(handlers, event, nil)
}

func (e *EventHandler) DispatchChannel[Event any](event Event) (<-chan error, error) {
	handlers, err := e.getHandlers[Event]()
	if err != nil {
		return nil, fmt.Errorf("namespace event: %w", err)
	}
	out := make(chan error, len(handlers))
	return out, e.dispatch(handlers, event, out)
}

func (e *EventHandler) getHandlers[Event any]() ([]Listener[any], error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	name, err := name[Event]()
	if err != nil {
		return nil, fmt.Errorf("namespace event: %w", err)
	}

	handlers, ok := e.handlers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnregisteredEvent, name)
	}

	return handlers, nil
}

func (e *EventHandler) dispatch[Event any](
	handlers []Listener[any],
	event Event,
	ch chan error,
) error {
	name, err := name[Event]()
	if err != nil {
		return fmt.Errorf("namespace event: %w", err)
	}
	e.slog.Debug("received event", "event", name)

	var results *sync.WaitGroup
	if ch != nil {
		results = &sync.WaitGroup{}
	}

	for _, h := range handlers {
		e.wg.Add(1)
		if results != nil {
			results.Add(1)
		}
		e.events <- func() {
			defer e.wg.Done()
			if results != nil {
				defer results.Done()
			}
			defer func() {
				if r := recover(); r != nil {
					e.slog.Error("event listener panicked", "event", name, "panic", r)
				}
			}()

			e.slog.Debug("processing event", "event", name)
			ctx, cancel := context.WithTimeout(context.Background(), e.opts.HandlerTimeout)
			defer cancel()
			err := h(ctx, event)
			if err != nil {
				e.slog.ErrorContext(ctx, "event listener failed", "event", name, "error", err)
			}
			if ch != nil {
				ch <- err
			}
		}
	}

	if results != nil {
		go func() {
			results.Wait()
			close(ch)
		}()
	}

	return nil
}

func Dispatch[Evt any](evt Evt) error {
	if global == nil {
		panic("cannot process event on nil global")
	}
	return global.Dispatch(evt)
}

func DispatchChannel[Evt any](evt Evt) (<-chan error, error) {
	if global == nil {
		panic("cannot process event on nil global")
	}
	return global.DispatchChannel(evt)
}

func (e *EventHandler) Run(ctx context.Context) {
	for range runtime.NumCPU() {
		e.running.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-e.closed:
					return
				case evt := <-e.events:
					evt()
				}
			}
		})
	}
}

// Flush stops the event handler and waits for all events in memory to be processed,
// and for every worker goroutine started by Run to have exited.
func (e *EventHandler) Flush() {
	close(e.closed)
	go func() {
		for {
			select {
			case f := <-e.events:
				f()
			default:
				return
			}
		}
	}()
	e.wg.Wait()
	e.running.Wait()
}

// Reset sets the handler up for another call to Run(). Only call this after Flush()
// has returned, otherwise workers from the previous Run() are still reading e.closed.
func (e *EventHandler) Reset() {
	e.closed = make(chan struct{})
}

var (
	ErrUnknownType = errors.New("unknown type")
)

func name[T any]() (string, error) {
	typeOf := reflect.TypeFor[T]()
	if typeOf.Name() != "" {
		return fmt.Sprintf("%s/%s", typeOf.PkgPath(), typeOf.Name()), nil
	}

	if typeOf.Kind() == reflect.Pointer {
		typeOfPtr := typeOf.Elem()
		return fmt.Sprintf("*%s.%s", typeOfPtr.PkgPath(), typeOfPtr.Name()), nil
	}

	return "", ErrUnknownType
}
