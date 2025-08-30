package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type YamlExtractor[T any] struct {
	data []byte
}

func NewYamlExtractor[T any](data []byte) YamlExtractor[T] {
	return YamlExtractor[T]{
		data: data,
	}
}

func (y YamlExtractor[T]) Extract(conf *T) error {
	if err := yaml.Unmarshal(y.data, conf); err != nil {
		return fmt.Errorf("unmarshal yaml to config: %w", err)
	}
	return nil
}

var _ Extractor[any] = YamlExtractor[any]{}
