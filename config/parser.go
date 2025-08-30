// Package config
package config

import (
	"fmt"
)

type Extractor[T any] interface {
	Extract(*T) error
}

type Parser[T any] struct {
	extractors []Extractor[T]
}

func NewParser[T any]() *Parser[T] {
	return &Parser[T]{
		extractors: []Extractor[T]{},
	}
}

func (p *Parser[T]) WithExtractors(e ...Extractor[T]) *Parser[T] {
	p.extractors = append(p.extractors, e...)
	return p
}

func (p Parser[T]) Parse() (T, error) {
	var conf T
	for _, e := range p.extractors {
		if err := e.Extract(&conf); err != nil {
			return conf, fmt.Errorf("run extractor: %w", err)
		}
	}
	return conf, nil
}
