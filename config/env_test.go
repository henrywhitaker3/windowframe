package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItParsesEnv(t *testing.T) {
	e := NewEnvExtractor[DummyConfig]()
	var conf DummyConfig
	t.Setenv("SOME_FIELD", "bongo")
	require.Nil(t, e.Extract(&conf))
	require.Equal(t, "bongo", conf.SomeField)
}
