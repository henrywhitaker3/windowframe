package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type DummyConfig struct {
	SomeField string `yaml:"some_field" env:"SOME_FIELD,overwrite" flag:"some-field"`
	SomeInt   int    `                                             flag:"some-int"`
	SomeInt8  int8   `                                             flag:"some-int-8"`
	SomeInt16 int16  `                                             flag:"some-int-16"`
}

func TestItParsesMultipleExtractors(t *testing.T) {
	p := NewParser[DummyConfig]()

	p.WithExtractors(NewYamlExtractor[DummyConfig]([]byte("some_field: bongo")))

	conf, err := p.Parse()
	require.Nil(t, err)
	require.Equal(t, "bongo", conf.SomeField)

	t.Setenv("SOME_FIELD", "apple")

	p.WithExtractors(NewEnvExtractor[DummyConfig]())

	conf, err = p.Parse()
	require.Nil(t, err)
	require.Equal(t, "apple", conf.SomeField)
}
