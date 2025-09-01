package test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	garageConfig = `metadata_dir = "/tmp/meta"
data_dir = "/tmp/data"
db_engine = "sqlite"

replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "3e844293abd869741d142cad93e269b1c9ff3c41240566508554c250d7668ec0"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[s3_web]
bind_addr = "[::]:3902"
root_domain = ".web.garage.localhost"
index = "index.html"

[k2v_api]
api_bind_addr = "[::]:3904"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "EVCNqzJY4StaQ7RGZ+triyhK6GCzgLNrhlqSvTMVyrI="
metrics_token = "neFhPdBSRjcrfTW4LKcnTMqJQAY6vOII+qdVQZK/Dtw="`
)

type S3Details struct {
	Port            int
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

func S3(t testing.TB) (S3Details, context.CancelFunc) {
	configPath := filepath.Join(t.TempDir(), "garage.toml")
	require.Nil(
		t,
		os.WriteFile(configPath, []byte(garageConfig), 0644),
	)
	req := testcontainers.ContainerRequest{
		Image:        "dxflrs/garage:v2.0.0",
		ExposedPorts: []string{"3900/tcp", "3901/tcp", "3902/tcp", "3903/tcp"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      configPath,
				ContainerFilePath: "/etc/garage.toml",
				FileMode:          0444,
			},
		},
		WaitingFor: &wait.HTTPStrategy{
			Port: "3903",
			Path: "/",
			StatusCodeMatcher: func(status int) bool {
				return status == 400
			},
		},
	}

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.Nil(t, err)

	body := runContainerCommand(t, container, []string{"/garage", "status"})
	nodeID := strings.Split(strings.Split(string(body), "\n")[4], " ")[0]
	t.Log("got node id", nodeID)

	runContainerCommand(
		t,
		container,
		[]string{"/garage", "layout", "assign", "-z", "test", "-c", "1G", nodeID},
	)
	runContainerCommand(t, container, []string{"/garage", "layout", "apply", "--version", "1"})

	port, err := container.MappedPort(ctx, "3900")
	require.Nil(t, err)

	bucket := "garagebucket"
	runContainerCommand(t, container, []string{"/garage", "bucket", "create", bucket})
	body = runContainerCommand(t, container, []string{"/garage", "key", "create", bucket})
	runContainerCommand(
		t,
		container,
		[]string{"/garage", "bucket", "allow", "--read", "--write", bucket, "--key", bucket},
	)

	spl := strings.Split(string(body), "\n")
	accessKeyID := strings.Trim(strings.Split(spl[3], ":")[1], " ")
	secretAccessKey := strings.Trim(strings.Split(spl[5], ":")[1], " ")

	return S3Details{
			Port:            port.Int(),
			Bucket:          bucket,
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
		}, func() {
			_ = container.Terminate(context.Background())
		}
}

func runContainerCommand(
	t testing.TB,
	container testcontainers.Container,
	command []string,
) []byte {
	rc, output, err := container.Exec(context.Background(), command)
	require.Nil(t, err)
	body, err := io.ReadAll(output)
	require.Nil(t, err)
	t.Log(string(body))
	require.Equal(t, 0, rc)
	return body
}
