// Package francis
package francis

import (
	"context"
	"time"

	"github.com/italypaleale/francis/actor"
	"github.com/italypaleale/francis/builtin/workflow"
)

type Invoker interface {
	Invoke(
		ctx context.Context,
		kind string,
		id string,
		method string,
		data any,
		opts ...actor.InvokeOption,
	) (actor.Envelope, error)
}

type Peeker interface {
	Peek(
		ctx context.Context,
		kind string,
		id string,
		data any,
		opts ...actor.InvokeOption,
	) (actor.Envelope, error)
}

type Workflower interface {
	Start(
		ctx context.Context,
		data any,
		opts ...workflow.StartOption,
	) (id string, started bool, err error)
}

type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

type Dispatcher interface {
	Dispatch(
		ctx context.Context,
		kind string,
		id string,
		method string,
		data any,
		opts ...actor.JobOption,
	) (jobID string, created bool, err error)
}
