package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestItExpiresRecords(t *testing.T) {
	cache := NewExpiringCache[string, string]()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	cache.Put(ctx, "bongo", "bingo", time.Minute)
	cache.Put(ctx, "bingo", "orange", time.Minute*5)

	val, ok := cache.Get(ctx, "bongo")
	require.True(t, ok)
	require.Equal(t, "bingo", val)
	val, ok = cache.Get(ctx, "bingo")
	require.True(t, ok)
	require.Equal(t, "orange", val)

	now = func() time.Time {
		return time.Now().Add(time.Minute * 3)
	}

	_, ok = cache.Get(ctx, "bongo")
	require.False(t, ok)
	val, ok = cache.Get(ctx, "bingo")
	require.True(t, ok)
	require.Equal(t, "orange", val)

	cache.Delete(ctx, "bingo")

	_, ok = cache.Get(ctx, "bingo")
	require.False(t, ok)
}
