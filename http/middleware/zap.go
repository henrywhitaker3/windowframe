package middleware

import (
	"log/slog"

	"github.com/henrywhitaker3/windowframe/v2/http/common"
	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/labstack/echo/v5"
)

func Zap(level slog.Level) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			id := common.RequestID(c)
			if id != "" {
				ctx = common.SetContextID(ctx, id)
			}

			if traceID := tracing.TraceID(ctx); traceID != "" {
				ctx = common.SetTraceID(ctx, traceID)
			}

			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
