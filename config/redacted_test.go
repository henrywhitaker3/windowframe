package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactedStringMarshalsAsRedacted(t *testing.T) {
	secret := RedactedString("do-not-log-me")

	text, err := secret.MarshalText()
	require.NoError(t, err)
	require.Equal(t, redactedValue, string(text))

	encoded, err := json.Marshal(secret)
	require.NoError(t, err)
	require.Equal(t, `"[REDACTED]"`, string(encoded))
}

func TestRedactedStringIsRedactedInSlogJSON(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))

	logger.Info("configured", "password", RedactedString("do-not-log-me"))

	require.Contains(t, out.String(), `"password":"[REDACTED]"`)
	require.NotContains(t, out.String(), "do-not-log-me")
}
