package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItParsesYaml(t *testing.T) {
	input := `
some_field: bongo`

	e := NewYamlExtractor[DummyConfig]([]byte(input))
	var conf DummyConfig
	require.Nil(t, e.Extract(&conf))
	require.Equal(t, "bongo", conf.SomeField)
}
