// Package interceptors
package interceptors

import (
	"context"
	"log/slog"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
)

func UnaryServerLogger(l *slog.Logger) grpc.UnaryServerInterceptor {
	return logging.UnaryServerInterceptor(
		logging.LoggerFunc(
			func(ctx context.Context, level logging.Level, msg string, fields ...any) {
				h := func(ctx context.Context, msg string, args ...any) {}
				switch level {
				case logging.LevelDebug:
					h = l.DebugContext
				case logging.LevelError:
					h = l.ErrorContext
				case logging.LevelInfo:
					h = l.InfoContext
				case logging.LevelWarn:
					h = l.WarnContext
				}
				h(ctx, msg, fields...)
			},
		),
	)
}
