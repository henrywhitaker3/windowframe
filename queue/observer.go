package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Observer struct {
	jobsProcessed        *prometheus.CounterVec
	jobsErrors           *prometheus.CounterVec
	jobsProcessedSeconds prometheus.HistogramVec

	jobsPushed       *prometheus.CounterVec
	jobsPushedErrors *prometheus.CounterVec

	logger *slog.Logger
}

type ObserverOpts struct {
	Logger *slog.Logger
	Reg    prometheus.Registerer
}

func NewObserver(opts ObserverOpts) *Observer {
	o := &Observer{
		jobsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_jobs_processed_total",
			Help: "The number of jobs processed by the consumer",
		}, []string{"task"}),
		jobsProcessedSeconds: *prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "queue_consumer_job_processing_time_seconds",
			Help: "The number of seconds taken for the job to be processed",
		}, []string{"task"}),
		jobsErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_consumer_job_errors_total",
			Help: "The number of errors when processing jobs",
		}, []string{"task"}),
		jobsPushed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_producer_jobs_pushed_total",
			Help: "The number of jobs pushed to the queue",
		}, []string{"queue", "task"}),
		jobsPushedErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "queue_producer_job_push_errors_total",
			Help: "The number of errors pushing a job to the queue",
		}, []string{"queue", "task"}),
		logger: opts.Logger,
	}

	if opts.Reg != nil {
		_ = opts.Reg.Register(o.jobsProcessed)
		_ = opts.Reg.Register(o.jobsProcessedSeconds)
		_ = opts.Reg.Register(o.jobsErrors)
		_ = opts.Reg.Register(o.jobsPushed)
		_ = opts.Reg.Register(o.jobsPushedErrors)
	}

	return o
}

func (o *Observer) observeSuccess(ctx context.Context, t Task, start time.Time) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.DebugContext(ctx, "job processed", "task", t, "duration", dur.String())
	}
	o.jobsProcessed.WithLabelValues(string(t)).Inc()
	o.jobsProcessedSeconds.WithLabelValues(string(t)).Observe(dur.Seconds())
}

func (o *Observer) observeError(ctx context.Context, t Task, start time.Time, err error) {
	dur := time.Since(start)
	if o.logger != nil {
		o.logger.ErrorContext(ctx, "job failed", "task", t, "duration", dur.String(), "error", err)
	}
	o.jobsProcessed.WithLabelValues(string(t)).Inc()
	o.jobsErrors.WithLabelValues(string(t)).Inc()
	o.jobsProcessedSeconds.WithLabelValues(string(t)).Observe(dur.Seconds())
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
