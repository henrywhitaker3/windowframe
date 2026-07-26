package middleware

import (
	"net/http"
	"slices"

	echootel "github.com/labstack/echo-opentelemetry"
	"github.com/labstack/echo/v5"
)

var (
	skipPaths = []string{"/metrics", "/"}
)

func Tracing(serviceName string) echo.MiddlewareFunc {
	mw, err := echootel.Config{
		// serviceName is not a host:port address, but echootel only exposes
		// ServerName as an identifying label, and uses it verbatim as the
		// server.address span/metric attribute when it doesn't parse as one.
		ServerName: serviceName,
		Skipper: func(c *echo.Context) bool {
			if c.Request().Method == http.MethodOptions {
				return true
			}
			if slices.Contains(skipPaths, c.Path()) {
				return true
			}
			return false
		},
	}.ToMiddleware()
	if err != nil {
		panic(err)
	}
	return mw
}
