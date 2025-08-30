// Package storage
package storage

import (
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/thanos-io/objstore"
	"github.com/thanos-io/objstore/providers/filesystem"
	"github.com/thanos-io/objstore/providers/s3"
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
)

func New(backend Backend, conf map[string]any) (objstore.Bucket, error) {
	switch backend {
	case S3:
		return newS3(conf)
	case Filesystem:
		return newFilesystem(conf)
	default:
		return nil, ErrUnsupportedStorageType
	}
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
