package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestSwaggerHandlerServesScalar(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, NewSwagger("https://api.example.com").Handler()(e.NewContext(req, rec)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get(echo.HeaderContentType), echo.MIMETextHTML)
	require.Contains(t, rec.Body.String(), `data-url="https://api.example.com/docs/schema.yaml"`)
	require.Contains(t, rec.Body.String(), `"agent":{"disabled":true}`)
	require.Contains(t, rec.Body.String(), `"darkMode":true`)
	require.Contains(t, rec.Body.String(), `"defaultHttpClient":{"targetKey":"go","clientKey":"native"}`)
	require.Contains(t, rec.Body.String(), `"showDeveloperTools":"never"`)
	require.Contains(t, rec.Body.String(), `src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"`)
}
