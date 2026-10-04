package log

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/henrywhitaker3/ctxgen"
)

type Handler struct {
	slog.Handler
}

func NewHandler(h slog.Handler) *Handler {
	return &Handler{
		Handler: h,
	}
}

type contextKey string

const (
	logAttrsKey contextKey = "attrs.map"
)

func getLogAttrs(ctx context.Context) map[string]any {
	val, _ := ctxgen.ValueOk[map[string]any](ctx, logAttrsKey)
	return val
}

func storeLogAttrs(ctx context.Context, attrs map[string]any) context.Context {
	return ctxgen.WithValue(ctx, logAttrsKey, attrs)
}

func WithLogAttr(ctx context.Context, key string, val any) context.Context {
	attrs := getLogAttrs(ctx)
	updatedAttrs := make(map[string]any, len(attrs)+1)
	maps.Copy(updatedAttrs, attrs)
	updatedAttrs[key] = val
	return storeLogAttrs(ctx, updatedAttrs)
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if req, ok := ctxgen.ValueOk[string](ctx, "request_id"); ok {
		record.AddAttrs(slog.String("request_id", req))
	}
	if trace, ok := ctxgen.ValueOk[string](ctx, "trace_id"); ok {
		record.AddAttrs(slog.String("trace_id", trace))
	}
	if user, ok := ctxgen.ValueOk[string](ctx, "user_id"); ok {
		record.AddAttrs(slog.String("user_id", user))
	}
	if team, ok := ctxgen.ValueOk[string](ctx, "team_id"); ok {
		record.AddAttrs(slog.String("team_id", team))
	}

	attrs := getLogAttrs(ctx)
	for key, val := range attrs {
		switch typed := val.(type) {
		case string:
			record.AddAttrs(slog.String(key, typed))
		case bool:
			record.AddAttrs(slog.Bool(key, typed))
		case int:
			record.AddAttrs(slog.Int64(key, int64(typed)))
		case int8:
			record.AddAttrs(slog.Int64(key, int64(typed)))
		case int16:
			record.AddAttrs(slog.Int64(key, int64(typed)))
		case int32:
			record.AddAttrs(slog.Int64(key, int64(typed)))
		case int64:
			record.AddAttrs(slog.Int64(key, typed))
		case uint:
			record.AddAttrs(slog.Uint64(key, uint64(typed)))
		case uint8:
			record.AddAttrs(slog.Uint64(key, uint64(typed)))
		case uint16:
			record.AddAttrs(slog.Uint64(key, uint64(typed)))
		case uint32:
			record.AddAttrs(slog.Uint64(key, uint64(typed)))
		case uint64:
			record.AddAttrs(slog.Uint64(key, typed))
		case float32:
			record.AddAttrs(slog.Float64(key, float64(typed)))
		case float64:
			record.AddAttrs(slog.Float64(key, typed))
		case time.Time:
			record.AddAttrs(slog.Time(key, typed))
		case time.Duration:
			record.AddAttrs(slog.Duration(key, typed))
		default:
			record.AddAttrs(slog.Any(key, typed))
		}
	}

	return h.Handler.Handle(ctx, record)
}

var _ slog.Handler = &Handler{}
