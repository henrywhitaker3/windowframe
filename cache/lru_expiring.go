package cache

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type LruCacheExpiring[T comparable, U any] struct {
	cache *lru.Cache[T, expiringItem[U]]
}

func NewLruCacheExpiring[T comparable, U any](size int) (*LruCacheExpiring[T, U], error) {
	cache, err := lru.New[T, expiringItem[U]](size)
	if err != nil {
		return nil, fmt.Errorf("init lru cache: %w", err)
	}
	return &LruCacheExpiring[T, U]{
		cache: cache,
	}, nil
}

func (l *LruCacheExpiring[T, U]) Get(ctx context.Context, key T) (U, bool) {
	ctx, span := newTrace(ctx, "GetKey", key)
	defer span.End()
	item, ok := l.cache.Get(key)
	if !ok {
		return item.item, ok
	}
	if item.expires.Before(now()) {
		l.Delete(ctx, key)
		return item.item, false
	}
	return item.item, true
}

func (l *LruCacheExpiring[T, U]) Put(ctx context.Context, key T, val U, validity time.Duration) {
	_, span := newTrace(ctx, "PutKey", key)
	defer span.End()
	l.cache.Add(key, expiringItem[U]{
		item:    val,
		expires: now().Add(validity),
	})
}

func (l *LruCacheExpiring[T, U]) Delete(ctx context.Context, key T) {
	_, span := newTrace(ctx, "Delete", key)
	defer span.End()
	l.cache.Remove(key)
}

func (l *LruCacheExpiring[T, U]) Len() int {
	return l.cache.Len()
}
