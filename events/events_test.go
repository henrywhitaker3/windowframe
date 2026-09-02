package events_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/henrywhitaker3/windowframe/v2/events"
	"github.com/stretchr/testify/require"
)

func TestDispatchUnregisteredEventReturnsError(t *testing.T) {
	type unregistered struct{}

	handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

	err := handler.Dispatch(unregistered{})

	require.ErrorIs(t, err, events.ErrUnregisteredEvent)
}

func TestDispatchChannelUnregisteredEventReturnsError(t *testing.T) {
	type unregistered struct{}

	handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

	results, err := handler.DispatchChannel(unregistered{})

	require.Nil(t, results)
	require.ErrorIs(t, err, events.ErrUnregisteredEvent)
}

func TestListenUnnamedTypeReturnsError(t *testing.T) {
	handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

	err := handler.Listen(func(ctx context.Context, e struct{ X int }) error {
		return nil
	})

	require.ErrorIs(t, err, events.ErrUnknownType)
}

func TestEventDispatchesToRegisteredListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type widgetCreated struct{ ID string }

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

		received := make(chan widgetCreated, 1)
		require.NoError(t, handler.Listen(func(ctx context.Context, e widgetCreated) error {
			received <- e
			return nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		require.NoError(t, handler.Dispatch(widgetCreated{ID: "abc"}))

		// Wait for the worker pool to become idle again, meaning the
		// dispatched event has been fully processed.
		synctest.Wait()

		select {
		case e := <-received:
			require.Equal(t, "abc", e.ID)
		default:
			t.Fatal("listener was not invoked")
		}

		handler.Flush()
	})
}

func TestDispatchChannelReturnsEveryListenerResultAndCloses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type widgetCreated struct{}
		listenerErr := errors.New("listener failed")

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})
		require.NoError(t, handler.Listen(
			func(context.Context, widgetCreated) error { return nil },
			func(context.Context, widgetCreated) error { return listenerErr },
		))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		results, err := handler.DispatchChannel(widgetCreated{})
		require.NoError(t, err)

		synctest.Wait()

		var got []error
		for result := range results {
			got = append(got, result)
		}
		require.ElementsMatch(t, []error{nil, listenerErr}, got)

		handler.Flush()
	})
}

func TestMultipleListenersAllInvoked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type pinged struct{}

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

		var calls atomic.Int32
		listener := func(ctx context.Context, e pinged) error {
			calls.Add(1)
			return nil
		}
		require.NoError(t, handler.Listen(listener, listener))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		require.NoError(t, handler.Dispatch(pinged{}))
		synctest.Wait()

		require.Equal(t, int32(2), calls.Load())

		handler.Flush()
	})
}

func TestListenerPanicIsRecoveredAndPoolKeepsProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type risky struct{ Boom bool }

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

		processed := make(chan bool, 1)
		require.NoError(t, handler.Listen(func(ctx context.Context, e risky) error {
			if e.Boom {
				panic("boom")
			}
			processed <- true
			return nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		// This event panics the listener. The worker pool must recover and
		// keep running rather than losing a worker.
		require.NoError(t, handler.Dispatch(risky{Boom: true}))
		synctest.Wait()

		require.NoError(t, handler.Dispatch(risky{Boom: false}))
		synctest.Wait()

		select {
		case <-processed:
		default:
			t.Fatal("handler pool stopped processing events after a listener panicked")
		}

		handler.Flush()
	})
}

func TestHandlerTimeoutCancelsListenerContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type slow struct{}

		const timeout = time.Millisecond
		handler := events.New(events.EventHandlerOptions{HandlerTimeout: timeout})

		result := make(chan error, 1)
		require.NoError(t, handler.Listen(func(ctx context.Context, e slow) error {
			<-ctx.Done()
			result <- ctx.Err()
			return nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		require.NoError(t, handler.Dispatch(slow{}))

		synctest.Sleep(timeout)

		select {
		case err := <-result:
			require.ErrorIs(t, err, context.DeadlineExceeded)
		default:
			t.Fatal("listener context was not cancelled after HandlerTimeout elapsed")
		}

		handler.Flush()
	})
}

func TestFlushProcessesEventsQueuedBeforeRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type queued struct{ N int }

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

		var processed atomic.Int32
		require.NoError(t, handler.Listen(func(ctx context.Context, e queued) error {
			processed.Add(1)
			return nil
		}))

		// Dispatch events without ever calling Run: Flush should still drain
		// and process everything sitting in the queue.
		for i := range 5 {
			require.NoError(t, handler.Dispatch(queued{N: i}))
		}

		handler.Flush()

		require.Equal(t, int32(5), processed.Load())
	})
}

func TestResetAllowsHandlerToRunAgainAfterFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type reopened struct{}

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})

		var processed atomic.Int32
		require.NoError(t, handler.Listen(func(ctx context.Context, e reopened) error {
			processed.Add(1)
			return nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		handler.Run(ctx)
		require.NoError(t, handler.Dispatch(reopened{}))
		synctest.Wait()
		handler.Flush()

		require.Equal(t, int32(1), processed.Load())

		handler.Reset()
		handler.Run(ctx)
		require.NoError(t, handler.Dispatch(reopened{}))
		synctest.Wait()
		handler.Flush()

		require.Equal(t, int32(2), processed.Load())
	})
}

func TestSetGlobalEnablesListenAndDispatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type globalEvent struct{ Msg string }

		handler := events.New(events.EventHandlerOptions{HandlerTimeout: time.Second})
		events.SetGlobal(handler)

		received := make(chan string, 1)
		require.NoError(t, events.Listen(func(ctx context.Context, e globalEvent) error {
			received <- e.Msg
			return nil
		}))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler.Run(ctx)

		require.NoError(t, events.Dispatch(globalEvent{Msg: "hi"}))
		synctest.Wait()

		select {
		case msg := <-received:
			require.Equal(t, "hi", msg)
		default:
			t.Fatal("global listener was not invoked")
		}

		results, err := events.DispatchChannel(globalEvent{Msg: "bye"})
		require.NoError(t, err)
		synctest.Wait()

		require.NoError(t, <-results)
		_, open := <-results
		require.False(t, open)

		select {
		case msg := <-received:
			require.Equal(t, "bye", msg)
		default:
			t.Fatal("global listener was not invoked through DispatchChannel")
		}

		handler.Flush()
	})
}
