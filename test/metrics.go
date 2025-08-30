package test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func GetCounterValue(t *testing.T, m prometheus.Counter) float64 {
	dto := &dto.Metric{}
	require.Nil(t, m.Write(dto))
	unty := dto.GetCounter()
	require.NotNil(t, unty)
	val := unty.Value
	require.NotNil(t, val)
	return *val
}

func GetGaugeValue(t *testing.T, m prometheus.Gauge) float64 {
	dto := &dto.Metric{}
	require.Nil(t, m.Write(dto))
	unty := dto.GetGauge()
	require.NotNil(t, unty)
	val := unty.Value
	require.NotNil(t, val)
	return *val
}
