package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/http/middleware"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/henrywhitaker3/windowframe/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/rueidis"
	"github.com/stretchr/testify/require"
)

func TestItOnlyCallsHandlerOnce(t *testing.T) {
	_, e, hits, cancel := setup(t, middleware.IdempotentOpts{
		SkipMissingKeys: middleware.Ptr(false),
	})
	defer cancel()

	// Should error as we don't skip missing keys
	t.Log("running request with no key")
	do(t, e, uuid.UUID{}, http.StatusInternalServerError)
	require.Equal(t, int32(0), hits.Load())

	id := uuid.MustNew()
	t.Log("running first request with a key")
	first := do(t, e, id)
	require.Equal(t, int32(1), hits.Load())
	t.Log("running second request with a key")
	second := do(t, e, id)
	require.Equal(t, int32(1), hits.Load())

	require.Equal(t, first.Body.String(), second.Body.String())
	for key := range first.Header() {
		t.Logf("checking header %s: %s", key, first.Header().Get(key))
		require.Equal(t, first.Header().Get(key), second.Header().Get(key))
	}
}

func TestItRespondsWithALockedStatusForALockedRequest(t *testing.T) {
	store, e, hits, cancel := setup(t, middleware.IdempotentOpts{})
	defer cancel()

	id := uuid.MustNew()

	require.Nil(t, store.Lock(context.Background(), id, time.Minute))

	do(t, e, id, http.StatusLocked)
	require.Equal(t, int32(0), hits.Load())
}

func TestItRunsWithNoKey(t *testing.T) {
	_, e, hits, cancel := setup(t, middleware.IdempotentOpts{})
	defer cancel()

	do(t, e, uuid.UUID{})
	do(t, e, uuid.UUID{})
	require.Equal(t, int32(2), hits.Load())
}

func TestItReturnsNewResponseWithDifferentKey(t *testing.T) {
	_, e, hits, cancel := setup(t, middleware.IdempotentOpts{})
	defer cancel()

	first := do(t, e, uuid.MustNew())
	second := do(t, e, uuid.MustNew())

	require.Equal(t, int32(2), hits.Load())

	require.NotEqual(t, first.Header().Get("random-id"), second.Header().Get("random-id"))
}

func TestItReturnsNewResponseAfterTTLHasPassed(t *testing.T) {
	_, e, hits, cancel := setup(t, middleware.IdempotentOpts{
		TTL: time.Second * 2,
	})
	defer cancel()

	id := uuid.MustNew()
	first := do(t, e, id)
	second := do(t, e, id)

	require.Equal(t, int32(1), hits.Load())
	require.Equal(t, first.Header().Get("random-id"), second.Header().Get("random-id"))

	time.Sleep(time.Second * 3)

	third := do(t, e, id)

	require.Equal(t, int32(2), hits.Load())
	require.NotEqual(t, first.Header().Get("random-id"), third.Header().Get("random-id"))
}

func setup(
	t testing.TB,
	opts middleware.IdempotentOpts,
) (middleware.IdempotencyStore, *echo.Echo, *atomic.Int32, context.CancelFunc) {
	hits := &atomic.Int32{}

	port, cancel := test.Redis(t)

	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{fmt.Sprintf("127.0.0.1:%d", port)},
	})
	require.Nil(t, err)

	store := middleware.NewIdempotentRueidisStore(client)

	opts.Store = store

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		hits.Add(1)
		c.Response().Header().Set("random-id", uuid.MustNew().String())
		return c.String(http.StatusOK, "hello")
	}, middleware.Idempotent(opts))

	return store, e, hits, cancel
}

func do(t testing.TB, e *echo.Echo, id uuid.UUID, codes ...int) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	var empty uuid.UUID
	if id != empty {
		req.Header.Set(middleware.IdempotencyHeaderKey, id.String())
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if len(codes) == 0 {
		codes = append(codes, http.StatusOK)
	}
	require.Equal(t, codes[0], rec.Code, rec.Body.String())
	return rec
}
