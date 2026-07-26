package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/henrywhitaker3/windowframe/v2/http/common"
	"github.com/henrywhitaker3/windowframe/v2/test"
	"github.com/labstack/echo-contrib/v5/echoprometheus"
	"github.com/labstack/echo/v5"
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

	Register(srv, &dummyHandler{
		err: fmt.Errorf("some error"),
	})

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
		`test_request_total{code="500",host="example.com",method="GET",url="/"} 1`,
	)
}

func TestItReturnsValidationErrorsProperly(t *testing.T) {
	srv := New(HTTPOpts{
		Port:   0,
		Logger: test.NewLogger(t),
	})
	reg := prometheus.NewRegistry()
	srv.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Registerer: reg,
		Subsystem:  "test",
	}))

	Register(srv, &dummyHandler{})

	rec := httptest.NewRecorder()
	body, err := json.Marshal(dummyRequest{AField: "bongo"})
	require.Nil(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", bytes.NewReader(body))
	req.Header.Add("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	var buf bytes.Buffer
	require.Nil(t, echoprometheus.WriteGatheredMetrics(&buf, reg))
	require.NotContains(
		t,
		buf.String(),
		`test_request_total{code="422",host="example.com",method="GET",url="/"} 1`,
	)
}

type dummyHandler struct {
	err error
}

type dummyRequest struct {
	AField string `validate:"uppercase"`
}

func (d *dummyHandler) Handler() common.Handler[dummyRequest, any] {
	return func(c *echo.Context, req dummyRequest) (*any, error) {
		if d.err == nil {
			return nil, nil
		}
		return nil, d.err
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
