package queue

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

type ProducerObserver struct {
	JobsPushed       *prometheus.CounterVec
	JobsPushedErrors *prometheus.CounterVec

	logger *slog.Logger
}

type ProducerObserverOpts struct {
	Logger *slog.Logger
	Reg    prometheus.Registerer
}

func NewProducerObserver(opts ProducerObserverOpts) *ProducerObserver {
	o := &ProducerObserver{
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
		_ = opts.Reg.Register(o.JobsPushed)
		_ = opts.Reg.Register(o.JobsPushedErrors)
	}

	return o
}

func (o *ProducerObserver) observerJobPushed(ctx context.Context, is Task, opts []Option) {
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

func (o *ProducerObserver) observerJobPushedError(
	ctx context.Context,
	is Task,
	opts []Option,
	err error,
) {
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
