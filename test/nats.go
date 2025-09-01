package test

import (
	"context"
	"testing"
	"time"

	nnats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/nats"
)

func Nats(t testing.TB) (string, context.CancelFunc) {
	nats, err := nats.Run(
		context.Background(),
		"nats:2.11.8",
		nats.WithArgument("store_dir", t.TempDir()),
		testcontainers.WithCmdArgs("--jetstream"),
		testcontainers.WithLogger(log.TestLogger(t)),
	)
	require.Nil(t, err)

	url, err := nats.ConnectionString(context.Background())
	require.Nil(t, err)

	time.Sleep(time.Millisecond * 100)

	return url, func() {
		_ = nats.Terminate(context.Background())
	}
}

func NatsStream(t testing.TB, url string, conf jetstream.StreamConfig) {
	conn, err := nnats.Connect(url)
	require.Nil(t, err)

	js, err := jetstream.New(conn)
	require.Nil(t, err)

	_, err = js.CreateOrUpdateStream(context.Background(), conf)
	require.Nil(t, err)
}
