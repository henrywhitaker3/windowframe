package tracing

import (
	"errors"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessOwnerAttribute(t *testing.T) {
	t.Run("uses the current username", func(t *testing.T) {
		attr := processOwnerAttribute(func() (*user.User, error) {
			return &user.User{Username: "service"}, nil
		}, "10001")

		require.Equal(t, "process.owner", string(attr.Key))
		require.Equal(t, "service", attr.Value.AsString())
	})

	t.Run("uses the process UID when the user cannot be resolved", func(t *testing.T) {
		attr := processOwnerAttribute(func() (*user.User, error) {
			return nil, errors.New("user not found")
		}, "10001")

		require.Equal(t, "process.owner", string(attr.Key))
		require.Equal(t, "10001", attr.Value.AsString())
	})

	t.Run("uses the process UID when the resolved user has no username", func(t *testing.T) {
		attr := processOwnerAttribute(func() (*user.User, error) {
			return &user.User{}, nil
		}, "10001")

		require.Equal(t, "10001", attr.Value.AsString())
	})
}
