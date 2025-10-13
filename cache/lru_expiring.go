package cache

import (
	"context"
	"fmt"
	"time"
)

type LruCacheExpiring[T comparable, U any] struct {
	cache *LruCache[T, expiringItem[U]]
}

func NewLruCacheExpiring[T comparable, U any](size int) (*LruCacheExpiring[T, U], error) {
	cache, err := NewLruCache[T, expiringItem[U]](size)
	if err != nil {
		return nil, fmt.Errorf("init lru cache: %w", err)
	}
	return &LruCacheExpiring[T, U]{
		cache: cache,
	}, nil
}

func (l *LruCacheExpiring[T, U]) Get(ctx context.Context, key T, disableTracing ...bool) (U, bool) {
	item, ok := l.cache.Get(ctx, key, disableTracing...)
	if !ok {
		return item.item, ok
	}
	if item.expires.Before(now()) {
		l.Delete(ctx, key)
		return item.item, false
	}
	return item.item, true
}

func (l *LruCacheExpiring[T, U]) Put(
	ctx context.Context,
	key T,
	val U,
	validity time.Duration,
	disableTracing ...bool,
) {
	l.cache.Put(ctx, key, expiringItem[U]{
		item:    val,
		expires: now().Add(validity),
	}, disableTracing...)
}

func (l *LruCacheExpiring[T, U]) Delete(ctx context.Context, key T, disableTracing ...bool) {
	l.cache.Delete(ctx, key, disableTracing...)
}

func (l *LruCacheExpiring[T, U]) Len() int {
	return l.cache.Len()
}
