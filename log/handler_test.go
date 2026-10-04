package log

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type recordHandler struct {
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, record slog.Record) error {
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(string) slog.Handler { return h }

func TestHandlerSetsContextAttributesOnRecord(t *testing.T) {
	now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)
	attributes := []struct {
		key   string
		value any
		kind  slog.Kind
		check func(t *testing.T, value slog.Value)
	}{
		{"string", "value", slog.KindString, func(t *testing.T, value slog.Value) {
			if got := value.String(); got != "value" {
				t.Errorf("String() = %q, want %q", got, "value")
			}
		}},
		{"bool", true, slog.KindBool, func(t *testing.T, value slog.Value) {
			if got := value.Bool(); got != true {
				t.Errorf("Bool() = %t, want true", got)
			}
		}},
		{"int", int(-1), slog.KindInt64, func(t *testing.T, value slog.Value) {
			if got := value.Int64(); got != -1 {
				t.Errorf("Int64() = %d, want -1", got)
			}
		}},
		{"int8", int8(-8), slog.KindInt64, func(t *testing.T, value slog.Value) {
			if got := value.Int64(); got != -8 {
				t.Errorf("Int64() = %d, want -8", got)
			}
		}},
		{"int16", int16(-16), slog.KindInt64, func(t *testing.T, value slog.Value) {
			if got := value.Int64(); got != -16 {
				t.Errorf("Int64() = %d, want -16", got)
			}
		}},
		{"int32", int32(-32), slog.KindInt64, func(t *testing.T, value slog.Value) {
			if got := value.Int64(); got != -32 {
				t.Errorf("Int64() = %d, want -32", got)
			}
		}},
		{"int64", int64(-64), slog.KindInt64, func(t *testing.T, value slog.Value) {
			if got := value.Int64(); got != -64 {
				t.Errorf("Int64() = %d, want -64", got)
			}
		}},
		{"uint", uint(1), slog.KindUint64, func(t *testing.T, value slog.Value) {
			if got := value.Uint64(); got != 1 {
				t.Errorf("Uint64() = %d, want 1", got)
			}
		}},
		{"uint8", uint8(8), slog.KindUint64, func(t *testing.T, value slog.Value) {
			if got := value.Uint64(); got != 8 {
				t.Errorf("Uint64() = %d, want 8", got)
			}
		}},
		{"uint16", uint16(16), slog.KindUint64, func(t *testing.T, value slog.Value) {
			if got := value.Uint64(); got != 16 {
				t.Errorf("Uint64() = %d, want 16", got)
			}
		}},
		{"uint32", uint32(32), slog.KindUint64, func(t *testing.T, value slog.Value) {
			if got := value.Uint64(); got != 32 {
				t.Errorf("Uint64() = %d, want 32", got)
			}
		}},
		{"uint64", uint64(64), slog.KindUint64, func(t *testing.T, value slog.Value) {
			if got := value.Uint64(); got != 64 {
				t.Errorf("Uint64() = %d, want 64", got)
			}
		}},
		{"float32", float32(3.5), slog.KindFloat64, func(t *testing.T, value slog.Value) {
			if got := value.Float64(); got != 3.5 {
				t.Errorf("Float64() = %f, want 3.5", got)
			}
		}},
		{"float64", 6.5, slog.KindFloat64, func(t *testing.T, value slog.Value) {
			if got := value.Float64(); got != 6.5 {
				t.Errorf("Float64() = %f, want 6.5", got)
			}
		}},
		{"time", now, slog.KindTime, func(t *testing.T, value slog.Value) {
			if got := value.Time(); !got.Equal(now) {
				t.Errorf("Time() = %v, want %v", got, now)
			}
		}},
		{"duration", 2 * time.Second, slog.KindDuration, func(t *testing.T, value slog.Value) {
			if got := value.Duration(); got != 2*time.Second {
				t.Errorf("Duration() = %v, want %v", got, 2*time.Second)
			}
		}},
	}

	capture := &recordHandler{}
	logger := slog.New(NewHandler(capture))
	ctx := context.Background()
	for _, attribute := range attributes {
		ctx = WithLogAttr(ctx, attribute.key, attribute.value)
	}

	logger.InfoContext(ctx, "test message")

	if len(capture.records) != 1 {
		t.Fatalf("captured %d records, want 1", len(capture.records))
	}

	values := make(map[string]slog.Value, len(attributes))
	capture.records[0].Attrs(func(attr slog.Attr) bool {
		values[attr.Key] = attr.Value
		return true
	})
	for _, attribute := range attributes {
		value, ok := values[attribute.key]
		if !ok {
			t.Errorf("attribute %q was not added to the record", attribute.key)
			continue
		}
		if value.Kind() != attribute.kind {
			t.Errorf(
				"attribute %q Kind() = %v, want %v",
				attribute.key,
				value.Kind(),
				attribute.kind,
			)
		}
		attribute.check(t, value)
	}
}
