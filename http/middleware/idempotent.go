package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/henrywhitaker3/windowframe/http/validation"
	"github.com/henrywhitaker3/windowframe/tracing"
	"github.com/henrywhitaker3/windowframe/uuid"
	"github.com/labstack/echo/v4"
)

type IdempotencyStore interface {
	// Get retrieves the cached response.
	// Should return an ErrIdempotentMissing if nothing is in the cache.
	Get(ctx context.Context, key uuid.UUID) ([]byte, error)
	// Store records the response from the handler to return to subsequent
	// invocations of the handler for a given key.
	Store(ctx context.Context, key uuid.UUID, ttl time.Duration, response []byte) error
	// Lock prevents multiple invocations for the same key running simultaneously.
	// Should return ErrIdemptentLocked if the key is already locked.
	Lock(ctx context.Context, key uuid.UUID, ttl time.Duration) error
	// Unlock release the lock for a given key
	Unlock(ctx context.Context, key uuid.UUID) error
}

type IdempotentOpts struct {
	Store IdempotencyStore

	// SkipMissingKeys skips idempotency handling when there is no key provided (default: true)
	SkipMissingKeys *bool

	// TTL determines how long a given key is stored for (default: 24h)
	TTL time.Duration

	// LockTTL determines how long the lock lasts for in the event the handler errors
	// before it can unlock the key (default: 1m).
	LockTTL time.Duration
}

func setIdempotentOptsDefaults(opts IdempotentOpts) IdempotentOpts {
	if opts.TTL == 0 {
		opts.TTL = time.Hour * 24
	}
	if opts.LockTTL == 0 {
		opts.LockTTL = time.Minute
	}
	if opts.SkipMissingKeys == nil {
		opts.SkipMissingKeys = Ptr(true)
	}
	return opts
}

const (
	IdempotencyHeaderKey = "X-Idempotency-Key"
)

var (
	ErrIdempotentLocked  = errors.New("idempotency key already locked")
	ErrIdempotentMissing = errors.New("idempotency key not in cache")
)

func Idempotent(opts IdempotentOpts) echo.MiddlewareFunc {
	opts = setIdempotentOptsDefaults(opts)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, span := tracing.NewSpan(c.Request().Context(), "IdempotentMiddleware")
			defer span.End()

			keyRaw := c.Request().Header.Get(IdempotencyHeaderKey)
			if keyRaw == "" {
				if *opts.SkipMissingKeys {
					return next(c)
				}
				return validation.Build().With("idempotency_key", "missing key")
			}

			key, err := uuid.Parse(keyRaw)
			if err != nil {
				return validation.Build().With("idempotency_key", "invalid uuid")
			}

			lctx, span := tracing.NewSpan(ctx, "LockKey")
			defer span.End()
			if err := opts.Store.Lock(lctx, key, opts.LockTTL); err != nil {
				if errors.Is(err, ErrIdempotentLocked) {
					return c.NoContent(http.StatusLocked)
				}
				return fmt.Errorf("lock idempotency key: %w", err)
			}
			defer opts.Store.Unlock(context.Background(), key)
			span.End()

			ctx, span = tracing.NewSpan(ctx, "HandleRequest")
			defer span.End()

			// If we're here, then we've got the lock
			if resp, err := opts.Store.Get(ctx, key); err != nil {
				if !errors.Is(err, ErrIdempotentMissing) {
					return fmt.Errorf("get idempotency key value: %w", err)
				}
			} else {
				var recorded responseRecorder
				if err := json.Unmarshal(resp, &recorded); err != nil {
					return fmt.Errorf("unmarshal cached response: %w", err)
				}

				for key, val := range recorded.Headers {
					for _, ival := range val {
						c.Response().Header().Add(key, ival)
					}
				}

				if _, err := c.Response().Writer.Write(recorded.Body); err != nil {
					return fmt.Errorf("write cached body to response: %w", err)
				}
				c.Response().Writer.WriteHeader(recorded.Code)
				return nil
			}

			recorder := &responseRecorder{
				writer:  c.Response().Writer,
				Headers: http.Header{},
			}
			c.Response().Writer = recorder

			// There's nothing in the cache for this key
			if err := next(c); err != nil {
				return err
			}

			recorded, err := json.Marshal(recorder)
			if err != nil {
				return fmt.Errorf("marshal recorded response: %w", err)
			}

			if err := opts.Store.Store(ctx, key, opts.TTL, recorded); err != nil {
				return fmt.Errorf("store idempotency key value: %w", err)
			}

			return nil
		}
	}
}

type responseRecorder struct {
	writer http.ResponseWriter

	Code    int         `json:"code"`
	Body    []byte      `json:"body"`
	Headers http.Header `json:"headers"`
}

func (r *responseRecorder) Header() http.Header {
	return r.Headers
}

func (r *responseRecorder) WriteHeader(code int) {
	r.Code = code
	for key, val := range r.Headers {
		for _, ival := range val {
			r.writer.Header().Add(key, ival)
		}
	}
	r.writer.WriteHeader(code)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.Body = data
	return r.writer.Write(data)
}

var _ http.ResponseWriter = &responseRecorder{}

func Ptr[T any](v T) *T {
	return &v
}
