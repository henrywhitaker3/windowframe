package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

func Redis(t testing.TB) (int, context.CancelFunc) {
	redis, err := redis.Run(context.Background(), "redis:7")
	require.Nil(t, err)

	port, err := redis.MappedPort(context.Background(), "6379/tcp")
	require.Nil(t, err)

	return port.Int(), func() {
		_ = redis.Terminate(context.Background())
	}
}
