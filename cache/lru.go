package cache

import (
	"context"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
)

type LruCache[T comparable, U any] struct {
	cache *lru.Cache[T, U]
}

func NewLruCache[T comparable, U any](size int) (*LruCache[T, U], error) {
	cache, err := lru.New[T, U](size)
	if err != nil {
		return nil, fmt.Errorf("init lru cache: %w", err)
	}
	return &LruCache[T, U]{
		cache: cache,
	}, nil
}

func (l *LruCache[T, U]) Get(ctx context.Context, key T) (U, bool) {
	_, span := newTrace(ctx, "GetKey", key)
	defer span.End()
	return l.cache.Get(key)
}

func (l *LruCache[T, U]) Put(ctx context.Context, key T, val U) {
	_, span := newTrace(ctx, "PutKey", key)
	defer span.End()
	l.cache.Add(key, val)
}

func (l *LruCache[T, U]) Delete(ctx context.Context, key T) {
	_, span := newTrace(ctx, "Delete", key)
	defer span.End()
	l.cache.Remove(key)
}

func (l *LruCache[T, U]) Len() int {
	return l.cache.Len()
}
