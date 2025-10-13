// Package cache
package cache

import (
	"context"
	"time"
)

type FixedExpiryCache[T comparable, U any] struct {
	cache    *ExpiringCache[T, U]
	validity time.Duration
}

func NewFixedExpiryCache[T comparable, U any](validity time.Duration) *FixedExpiryCache[T, U] {
	return &FixedExpiryCache[T, U]{
		cache:    NewExpiringCache[T, U](),
		validity: validity,
	}
}

func (f *FixedExpiryCache[T, U]) Get(ctx context.Context, key T, disableTracing ...bool) (U, bool) {
	return f.cache.Get(ctx, key, disableTracing...)
}

func (f *FixedExpiryCache[T, U]) Put(ctx context.Context, key T, item U, disableTracing ...bool) {
	f.cache.Put(ctx, key, item, f.validity, disableTracing...)
}

func (f *FixedExpiryCache[T, U]) Len() int {
	return f.cache.Len()
}

func (f *FixedExpiryCache[T, U]) Delete(ctx context.Context, key T, disableTracing ...bool) {
	f.cache.Delete(ctx, key, disableTracing...)
}
