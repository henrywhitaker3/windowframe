package cache

import (
	"context"
	"sync"
	"time"
)

type expiringItem[T any] struct {
	item    T
	expires time.Time
}

type ExpiringCache[T comparable, U any] struct {
	items map[T]expiringItem[U]
	mu    *sync.RWMutex
}

func NewExpiringCache[T comparable, U any]() *ExpiringCache[T, U] {
	return &ExpiringCache[T, U]{
		items: map[T]expiringItem[U]{},
		mu:    &sync.RWMutex{},
	}
}

func (e *ExpiringCache[T, U]) Get(ctx context.Context, key T, enableTracing ...bool) (U, bool) {
	if len(enableTracing) > 0 && enableTracing[0] {
		_, span := newTrace(ctx, "GetKey", key)
		defer span.End()
	}

	e.mu.RLock()
	item, ok := e.items[key]
	e.mu.RUnlock()

	var empty U
	if !ok {
		return empty, false
	}

	if item.expires.Before(now()) {
		defer e.expire(key)
		return empty, false
	}

	return item.item, true
}

func (e *ExpiringCache[T, U]) Put(
	ctx context.Context,
	key T,
	item U,
	validity time.Duration,
	enableTracing ...bool,
) {
	if len(enableTracing) > 0 && enableTracing[0] {
		_, span := newTrace(ctx, "PutKey", key)
		defer span.End()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.items[key] = expiringItem[U]{
		item:    item,
		expires: now().Add(validity),
	}
}

func (e *ExpiringCache[T, U]) Delete(ctx context.Context, key T, enableTracing ...bool) {
	if len(enableTracing) > 0 && enableTracing[0] {
		_, span := newTrace(ctx, "DeleteKey", key)
		defer span.End()
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.items, key)
}

func (e *ExpiringCache[T, U]) Len() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.items)
}

func (e *ExpiringCache[T, U]) expire(key T) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.items, key)
}
