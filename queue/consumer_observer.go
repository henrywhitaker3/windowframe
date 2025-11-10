package queue

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
)

type ConsumerObserver struct {
	JobsProcessed        *prometheus.CounterVec
	JobsErrors           *prometheus.CounterVec
	JobsProcessedSeconds *prometheus.HistogramVec
	JobsDeadlettered     *prometheus.CounterVec
	JobsSkipped          *prometheus.CounterVec

	logger *slog.Logger
	sentry SentryOpts
}

type ConsumerObserverOpts struct {
	Logger *slog.Logger
	Reg    prometheus.Registerer
	Sentry SentryOpts
}

func NewConsumerObserver(opts ConsumerObserverOpts) *ConsumerObserver {
	o := &ConsumerObserver{
		JobsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_jobs_processed_total",
			Help: "The number of jobs processed by the consumer",
		}, []string{"task"}),
		JobsProcessedSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "queue_consumer_job_processing_time_seconds",
			Help: "The number of seconds taken for the job to be processed",
		}, []string{"task"}),
		JobsErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_job_errors_total",
			Help: "The number of errors when processing jobs",
		}, []string{"task"}),
		JobsDeadlettered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_jobs_deadlettered_total",
			Help: "The number of jobs that failed and were put on the deadletter queue",
		}, []string{"task"}),
		JobsSkipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_jobs_skipped_total",
			Help: "The number of jobs that failed and were skipped/removed from the queue",
		}, []string{"task"}),
		logger: opts.Logger,
		sentry: opts.Sentry,
	}

	if opts.Reg != nil {
		opts.Reg.MustRegister(o.JobsProcessed)
		_ = opts.Reg.Register(o.JobsProcessedSeconds)
		_ = opts.Reg.Register(o.JobsErrors)
		_ = opts.Reg.Register(o.JobsDeadlettered)
		_ = opts.Reg.Register(o.JobsSkipped)
	}

	o.sentry.Skipper = append(o.sentry.Skipper, func(err error) bool {
		return errors.Is(err, ErrDeadLetter)
	})
	o.sentry.Skipper = append(o.sentry.Skipper, func(err error) bool {
		return errors.Is(err, ErrSkipRetry)
	})

	return o
}

func (o *ConsumerObserver) observeStart(ctx context.Context, job Job) context.Context {
	if !o.sentry.Enabled {
		return ctx
	}

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	if client := hub.Client(); client != nil {
		client.SetSDKIdentifier("sentry.go.windowframe")
	}

	return sentry.SetHubOnContext(ctx, hub)
}

func (o *ConsumerObserver) observeSuccess(ctx context.Context, job Job, start time.Time) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.DebugContext(
			ctx,
			"job processed",
			"id",
			job.ID,
			"task",
			job.Task,
			"duration",
			dur.String(),
		)
	}
	o.JobsProcessed.WithLabelValues(string(job.Task)).Inc()
	o.JobsProcessedSeconds.WithLabelValues(string(job.Task)).Observe(dur.Seconds())
}

func (o *ConsumerObserver) observeError(ctx context.Context, job Job, start time.Time, err error) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.ErrorContext(
			ctx,
			"job failed",
			"id",
			job.ID,
			"task",
			job.Task,
			"duration",
			dur.String(),
			"error",
			err,
		)
	}
	o.JobsProcessed.WithLabelValues(string(job.Task)).Inc()
	o.JobsErrors.WithLabelValues(string(job.Task)).Inc()
	o.JobsProcessedSeconds.WithLabelValues(string(job.Task)).Observe(dur.Seconds())
	if errors.Is(err, ErrDeadLetter) {
		o.JobsDeadlettered.WithLabelValues(string(job.Task)).Inc()
	}
	if errors.Is(err, ErrSkipRetry) {
		o.JobsSkipped.WithLabelValues(string(job.Task)).Inc()
	}
	if o.sentry.Enabled {
		o.reportError(ctx, err)
	}
}
