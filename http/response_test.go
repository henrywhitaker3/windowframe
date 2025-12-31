package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/henrywhitaker3/windowframe/http/common"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestItReturnsJSONErrorsFromHandlers(t *testing.T) {
	srv := New(HTTPOpts{
		Port:   0,
		Logger: test.NewLogger(t),
	})
	srv.Use(echoprometheus.NewMiddleware("test"))

	srv.HandleErrors(func(err error) (int, any, bool) {
		return http.StatusForbidden, map[string]string{"message": "forbidden"}, true
	})

	Register(srv, &dummyHandler{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, `{"message":"forbidden"}
`, rec.Body.String())
	// Check the error handler overwrites metric status codes properly
	var buf bytes.Buffer
	require.Nil(t, echoprometheus.WriteGatheredMetrics(&buf, prometheus.DefaultGatherer))
	require.NotContains(
		t,
		buf.String(),
		`test_request_size_bytes_count{code="500",host="example.com",method="GET",url="/"} 1`,
	)
}

type dummyHandler struct{}

func (d *dummyHandler) Handler() common.Handler[any, any] {
	return func(c echo.Context, req any) (*any, error) {
		return nil, fmt.Errorf("some error")
	}
}

func (d *dummyHandler) Middleware() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{}
}

func (d *dummyHandler) Metadata() common.Metadata {
	return common.Metadata{
		Path:   "/",
		Method: http.MethodGet,
		Code:   http.StatusOK,
	}
}
