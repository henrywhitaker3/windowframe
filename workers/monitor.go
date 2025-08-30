package workers

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Monitor struct {
	executions *prometheus.CounterVec
	failures   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

type MonitorOpts struct {
	Namespace string
}

var (
	labels = []string{"name"}
)

func NewMonitor(opts MonitorOpts) *Monitor {
	return &Monitor{
		executions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "worker_exections_total",
			Namespace: opts.Namespace,
		}, labels),
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "worker_errors_total",
			Namespace: opts.Namespace,
		}, labels),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "worker_duration_seconds",
			Namespace: opts.Namespace,
		}, labels),
	}
}

func (m *Monitor) register(reg prometheus.Registerer) {
	_ = reg.Register(m.executions)
	_ = reg.Register(m.failures)
	_ = reg.Register(m.duration)
}

func (m *Monitor) run(name string, f func() error) {
	start := time.Now()
	err := f()
	dur := time.Since(start)
	m.executions.WithLabelValues(name).Inc()
	m.duration.WithLabelValues(name).Observe(float64(dur))
	if err != nil {
		m.failures.WithLabelValues(name).Inc()
	}
}
