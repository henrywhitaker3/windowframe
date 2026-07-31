# Events

An in-process, type-safe event dispatcher. Listeners are registered against a
Go type; dispatched events are matched to listeners by that type and run
asynchronously on a worker pool.

## Usage

```go
type UserCreated struct {
    ID string
}

handler := events.New(events.EventHandlerOptions{
    HandlerTimeout: 5 * time.Second,
})

events.Register(handler, func(ctx context.Context, e UserCreated) error {
    fmt.Println("user created:", e.ID)
    return nil
})

// Starts a pool of runtime.NumCPU() workers that process queued events.
handler.Run(context.Background())

if err := events.Event[UserCreated](handler, UserCreated{ID: "123"}); err != nil {
    panic(err)
}

// Waits for every queued event to be processed and every worker to exit.
handler.Flush()
```

You can register multiple listeners for the same event type, and they all run
independently:

```go
events.Register(handler, sendWelcomeEmail, provisionAccount)
```

## Global handler

For code that doesn't want `*EventHandler` threaded through as a dependency,
set a package-level handler once at startup and dispatch through it:

```go
events.SetGlobal(handler)

// register a listener without a reference to handler
events.Listen(func(ctx context.Context, e UserCreated) error {
    fmt.Println("user created:", e.ID)
    return nil
})

// dispatch an event without a reference to handler
if err := events.Dispatch(UserCreated{ID: "123"}); err != nil {
    panic(err)
}
```

`Listen` and `Dispatch` both panic if called before `SetGlobal`.

## Behaviour to be aware of

- **Async**: `Event`/`Dispatch` only enqueue the work — they return before any
  listener runs. Each event gets its own `context.Context` (independent of the
  caller's), cancelled after `HandlerTimeout`.
- **Errors are logged, not returned**: a listener's returned error is logged
  via `slog` and does not propagate back to the `Event`/`Dispatch` caller.
- **Panics are recovered**: a panicking listener is logged and does not take
  down the process or other listeners.
- **Unregistered events error**: dispatching a type with no registered
  listener returns `ErrUnregisteredEvent` immediately (this part is
  synchronous).
- **Restarting**: after `Flush()` returns, call `Reset()` before calling
  `Run()` again on the same handler.
