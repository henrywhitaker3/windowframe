package queue

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Observer struct {
	JobsProcessed        *prometheus.CounterVec
	JobsErrors           *prometheus.CounterVec
	JobsProcessedSeconds *prometheus.HistogramVec
	JobsDeadlettered     *prometheus.CounterVec
	JobsSkipped          *prometheus.CounterVec

	JobsPushed       *prometheus.CounterVec
	JobsPushedErrors *prometheus.CounterVec

	logger *slog.Logger
}

type ObserverOpts struct {
	Logger *slog.Logger
	Reg    prometheus.Registerer
}

func NewObserver(opts ObserverOpts) *Observer {
	o := &Observer{
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
		JobsPushed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_producer_jobs_pushed_total",
			Help: "The number of jobs pushed to the queue",
		}, []string{"queue", "task"}),
		JobsPushedErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_producer_job_push_errors_total",
			Help: "The number of errors pushing a job to the queue",
		}, []string{"queue", "task"}),
		logger: opts.Logger,
	}

	if opts.Reg != nil {
		_ = opts.Reg.Register(o.JobsProcessed)
		_ = opts.Reg.Register(o.JobsProcessedSeconds)
		_ = opts.Reg.Register(o.JobsErrors)
		_ = opts.Reg.Register(o.JobsPushed)
		_ = opts.Reg.Register(o.JobsPushedErrors)
		_ = opts.Reg.Register(o.JobsDeadlettered)
		_ = opts.Reg.Register(o.JobsSkipped)
	}

	return o
}

func (o *Observer) observeSuccess(ctx context.Context, t Task, start time.Time) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.DebugContext(ctx, "job processed", "task", t, "duration", dur.String())
	}
	o.JobsProcessed.WithLabelValues(string(t)).Inc()
	o.JobsProcessedSeconds.WithLabelValues(string(t)).Observe(dur.Seconds())
}

func (o *Observer) observeError(ctx context.Context, t Task, start time.Time, err error) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.ErrorContext(ctx, "job failed", "task", t, "duration", dur.String(), "error", err)
	}
	o.JobsProcessed.WithLabelValues(string(t)).Inc()
	o.JobsErrors.WithLabelValues(string(t)).Inc()
	o.JobsProcessedSeconds.WithLabelValues(string(t)).Observe(dur.Seconds())
	if errors.Is(err, ErrDeadLetter) {
		o.JobsDeadlettered.WithLabelValues(string(t)).Inc()
	}
	if errors.Is(err, ErrSkipRetry) {
		o.JobsSkipped.WithLabelValues(string(t)).Inc()
	}
}

func (o *Observer) observerJobPushed(ctx context.Context, is Task, opts []Option) {
	if o.logger != nil {
		o.logger.DebugContext(
			ctx,
			"job pushed",
			"queue",
			QueueFromOptions(opts),
			"task",
			is,
			"options",
			opts,
		)
	}
}

func (o *Observer) observerJobPushedError(ctx context.Context, is Task, opts []Option, err error) {
	if o.logger != nil {
		o.logger.ErrorContext(
			ctx,
			"job push failed",
			"queue",
			QueueFromOptions(opts),
			"task",
			is,
			"error",
			err,
		)
	}
}
