package storage_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/storage"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

func TestItStoresFilesInFilesystem(t *testing.T) {
	dir := t.TempDir()
	storage, err := storage.New(storage.Filesystem, map[string]any{"dir": dir})
	require.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	name := test.Word()
	contents := test.Sentence(15)

	require.Nil(t, storage.Upload(ctx, name, strings.NewReader(contents)))

	file, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, name))
	require.Nil(t, err)
	require.Equal(t, contents, string(file))
}

func TestItStoresFilesInS3(t *testing.T) {
	details, cancel := test.S3(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	storage, err := storage.New(storage.S3, map[string]any{
		"endpoint":   fmt.Sprintf("127.0.0.1:%d", details.Port),
		"region":     "garage",
		"bucket":     details.Bucket,
		"access_key": details.AccessKeyID,
		"secret_key": details.SecretAccessKey,
		"insecure":   true,
	})
	require.Nil(t, err)

	name := test.Word()
	contents := test.Sentence(15)

	require.Nil(t, storage.Upload(ctx, name, strings.NewReader(contents)))

	file, err := storage.Get(ctx, name)
	require.Nil(t, err)
	body, err := io.ReadAll(file)
	require.Nil(t, err)
	require.Equal(t, contents, string(body))
}
