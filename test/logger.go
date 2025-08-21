package test

import (
	"log/slog"
	"testing"
)

type testingWriter struct {
	t *testing.T
}

func (t testingWriter) Write(in []byte) (int, error) {
	t.t.Log(string(in))
	return len(in), nil
}

func NewLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewJSONHandler(&testingWriter{t: t}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
