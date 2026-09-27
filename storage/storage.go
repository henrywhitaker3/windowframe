// Package storage
package storage

import (
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/thanos-io/objstore"
	"github.com/thanos-io/objstore/providers/filesystem"
	"github.com/thanos-io/objstore/providers/s3"
	"github.com/thanos-io/objstore/tracing/opentelemetry"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

var (
	ErrUnsupportedStorageType = errors.New("storage type is not supported")
	ErrInvalidRootDir         = errors.New("invalid filesystem root directory")
)

type Backend int

const (
	S3 Backend = iota
	Filesystem
	Memory
)

func New(backend Backend, conf map[string]any) (objstore.Bucket, error) {
	var bucket objstore.Bucket
	var err error
	switch backend {
	case S3:
		bucket, err = newS3(conf)
	case Filesystem:
		bucket, err = newFilesystem(conf)
	case Memory:
		bucket, err = newMemory()
	default:
		return nil, ErrUnsupportedStorageType
	}

	if err != nil {
		return nil, err
	}

	if val, ok := conf["tracer"]; ok {
		if tracer, ok := val.(trace.Tracer); ok {
			bucket = opentelemetry.WrapWithTraces(bucket, tracer)
		}
	}

	return bucket, nil
}

func newS3(conf map[string]any) (objstore.Bucket, error) {
	by, err := yaml.Marshal(conf)
	if err != nil {
		return nil, err
	}
	return s3.NewBucket(log.NewNopLogger(), by, "storage", nil)
}

func newFilesystem(conf map[string]any) (objstore.Bucket, error) {
	dir, ok := conf["dir"]
	if !ok {
		return nil, fmt.Errorf("root dir not set: %w", ErrInvalidRootDir)
	}
	rootDir, ok := dir.(string)
	if !ok {
		return nil, fmt.Errorf("root dir not a string: %w", ErrInvalidRootDir)
	}
	bucket, err := filesystem.NewBucket(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open filesystem bucket: %w", err)
	}
	return bucket, nil
}

func newMemory() (objstore.Bucket, error) {
	return objstore.NewInMemBucket(), nil
}
