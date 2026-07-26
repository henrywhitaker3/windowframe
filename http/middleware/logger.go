// Package middleware
package middleware

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/henrywhitaker3/windowframe/tracing"
	"github.com/labstack/echo/v5"
)

type LogAttributesFunc = func(context.Context, *slog.Logger) *slog.Logger

func Logger(mut ...LogAttributesFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Echo().HTTPErrorHandler(c, err)
			}
			ctx, span := tracing.NewSpan(c.Request().Context(), "LogRequest")
			defer span.End()
			dur := time.Since(start)
			status, size := responseStatusAndSize(c)
			logger := slog.
				With(
					"remote_ip", c.RealIP(),
					"host", c.Request().Host,
					"uri", c.Request().RequestURI,
					"method", c.Request().Method,
					"user_agent", c.Request().UserAgent(),
					"status", status,
					"latency", dur.Nanoseconds(),
					"latency_human", dur.String(),
					"bytes_in", bytesIn(c),
					"bytes_out", strconv.FormatInt(size, 10),
				)
			if traceID := tracing.TraceID(ctx); traceID != "" {
				logger = logger.With("trace_id", traceID)
			}

			for _, f := range mut {
				logger = f(ctx, logger)
			}

			if err != nil {
				if status >= 500 {
					logger = logger.With("error", err.Error())
				}
			}
			logger.InfoContext(ctx, "request")
			return nil
		}
	}
}

func bytesIn(c *echo.Context) string {
	cl := c.Request().Header.Get(echo.HeaderContentLength)
	if cl == "" {
		cl = "0"
	}
	return cl
}

func responseStatusAndSize(c *echo.Context) (int, int64) {
	resp, err := echo.UnwrapResponse(c.Response())
	if err != nil {
		return 0, 0
	}
	return resp.Status, resp.Size
}
