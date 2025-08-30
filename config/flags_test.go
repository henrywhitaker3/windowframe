package config

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestItParsesPFlag(t *testing.T) {
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	set.String("some-field", "", "An example flag")
	set.Int("some-int", 0, "An int flag")
	set.Int8("some-int-8", 0, "An int8 flag")
	set.Int16("some-int-16", 0, "An int16 flag")
	set.Int32("some-int-32", 0, "An int32 flag")
	set.Int64("some-int-64", 0, "An int64 flag")
	set.Float32("some-float-32", 0, "A float32 flag")
	set.Float64("some-float-64", 0, "A float64 flag")
	set.Duration("some-duration", 0, "A duration flag")
	require.Nil(t, set.Parse([]string{
		"--some-field", "bongo",
		"--some-int", "6",
		"--some-int-8", "7",
		"--some-int-16", "8",
		"--some-int-32", "9",
		"--some-int-64", "10",
		"--some-float-32", "1.1",
		"--some-float-64", "1.2",
		"--some-duration", "1s",
	}))

	e := NewPFlagExtractor[DummyConfig](set)
	var conf DummyConfig
	require.Nil(t, e.Extract(&conf))
	require.Equal(t, "bongo", conf.SomeField)
	require.Equal(t, 6, conf.SomeInt)
	require.Equal(t, int8(7), conf.SomeInt8)
	require.Equal(t, int16(8), conf.SomeInt16)
	require.Equal(t, int32(9), conf.SomeInt32)
	require.Equal(t, int64(10), conf.SomeInt64)
	require.Equal(t, float32(1.1), conf.SomeFloat32)
	require.Equal(t, float64(1.2), conf.SomeFloat64)
	require.Equal(t, time.Second, conf.SomeDuration)
}

func TestItErrorsWhenPfldagsNotParsedYet(t *testing.T) {
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	set.String("some-field", "", "An example flag")

	e := NewPFlagExtractor[DummyConfig](set)
	var conf DummyConfig
	require.NotNil(t, e.Extract(&conf))
}
