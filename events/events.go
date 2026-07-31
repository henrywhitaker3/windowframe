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

func Listen[Event any](lis ...Listener[Event]) error {
	if global == nil {
		panic("cannot process event on nil global")
	}
	return Register(global, lis...)
}

func Register[Event any](handler *EventHandler, lis ...Listener[Event]) error {
	name, err := name[Event]()
	if err != nil {
		return fmt.Errorf("namespace event: %w", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	handler.slog.Debug("registering event", "event", name, "listeners", len(lis))

	handler.handlers[name] = append(handler.handlers[name], wrapListeners(lis...)...)

	return nil
}

var (
	ErrUnregisteredEvent = errors.New("unregistered event")
)

func Event[Event any](handler *EventHandler, event any) error {
	handler.mu.RLock()
	name, err := name[Event]()
	if err != nil {
		handler.mu.RUnlock()
		return fmt.Errorf("namespace event: %w", err)
	}

	handlers, ok := handler.handlers[name]
	if !ok {
		handler.mu.RUnlock()
		return fmt.Errorf("%w: %s", ErrUnregisteredEvent, name)
	}
	handler.mu.RUnlock()

	handler.slog.Debug("received event", "event", name)

	for _, h := range handlers {
		handler.wg.Add(1)
		handler.events <- func() {
			defer handler.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					handler.slog.Error("event listener panicked", "event", name, "panic", r)
				}
			}()

			handler.slog.Debug("processing event", "event", name)
			ctx, cancel := context.WithTimeout(context.Background(), handler.opts.HandlerTimeout)
			defer cancel()
			if err := h(ctx, event); err != nil {
				handler.slog.ErrorContext(ctx, "event listener failed", "event", name, "error", err)
			}
		}
	}

	return nil
}

func Dispatch[Evt any](evt Evt) error {
	if global == nil {
		panic("cannot process event on nil global")
	}
	return Event[Evt](global, evt)
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
