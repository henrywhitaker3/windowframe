package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type EnvExtractor[T any] struct {
	muts []envconfig.Mutator
}

func NewEnvExtractor[T any](mutators ...envconfig.Mutator) EnvExtractor[T] {
	return EnvExtractor[T]{
		muts: mutators,
	}
}

func (e EnvExtractor[T]) Extract(conf *T) error {
	if err := envconfig.Process(context.Background(), conf, e.muts...); err != nil {
		return fmt.Errorf("process env: %w", err)
	}
	return nil
}

var _ Extractor[any] = EnvExtractor[any]{}
