package test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestItGetsCounterValues(t testing.TB) {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bongo",
	})

	reg := prometheus.NewRegistry()
	require.Nil(t, reg.Register(counter))

	counter.Add(5)

	require.Equal(t, float64(5), GetCounterValue(t, counter))
}

func TestItGetsGaugeValues(t testing.TB) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bongo",
	})

	reg := prometheus.NewRegistry()
	require.Nil(t, reg.Register(gauge))

	gauge.Add(5)

	require.Equal(t, float64(5), GetGaugeValue(t, gauge))
}
