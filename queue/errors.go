package queue

import (
	"context"
	"errors"

	"github.com/getsentry/sentry-go"
)

var (
	// Stops retrying the task, doesn't archive
	ErrSkipRetry = errors.New("skip retry")
	// Stops retrying the task, saves it in an archive
	ErrDeadLetter = errors.New("deadletter")
)

type SentryOpts struct {
	// Enabled controls whether errors returned from handlers are reported to sentry
	Enabled bool

	// Skipper is a slice of functions that return true when the error should not be reported
	Skipper []func(error) bool
}

func (o *ConsumerObserver) reportError(ctx context.Context, err error) {
	if !o.sentry.Enabled {
		return
	}
	for _, s := range o.sentry.Skipper {
		if s(err) {
			return
		}
	}

	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.CaptureException(err)
	}
}
