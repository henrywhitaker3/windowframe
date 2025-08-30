// Package redis
package redis

import (
	"context"
	"strings"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisotel"
)

type RedisOpts struct {
	Addr           string
	Password       string
	MaxFlushDelay  time.Duration
	TracingEnabled bool
}

func New(ctx context.Context, opts RedisOpts) (rueidis.Client, error) {
	copts := rueidis.ClientOption{
		InitAddress: []string{opts.Addr},
		Password:    opts.Password,
	}
	if opts.MaxFlushDelay != 0 {
		copts.MaxFlushDelay = opts.MaxFlushDelay
	}

	var client rueidis.Client
	var err error
	if opts.TracingEnabled {
		client, err = rueidisotel.NewClient(
			copts,
			rueidisotel.WithDBStatement(func(cmdTokens []string) string {
				return strings.Join(cmdTokens, " ")
			}),
		)
	} else {
		client, err = rueidis.NewClient(copts)
	}

	return client, err
}

// Does a write to redis every second to check we can write, if it can't it marks the
// the app as unhealthy
func CheckCanWrite(ctx context.Context, client rueidis.Client, f func(status bool)) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			cmd := client.B().Set().Key("redis:can-write").Value("true").Build()
			if res := client.Do(ctx, cmd); res.Error() != nil {
				f(false)
				continue
			}
			f(true)
		}
	}
}
