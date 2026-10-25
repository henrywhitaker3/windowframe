package docs

import (
	"fmt"
	"html"
	"net/http"

	"github.com/labstack/echo/v5"
)

const scalarBundleURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference"

const scalarConfiguration = `{"agent":{"disabled":true},"darkMode":true,"defaultHttpClient":{"targetKey":"go","clientKey":"native"},"showDeveloperTools":"never"}`

type SwaggerHandler struct {
	url string
}

func NewSwagger(url string) *SwaggerHandler {
	return &SwaggerHandler{
		url: url,
	}
}

func (s *SwaggerHandler) Handler() echo.HandlerFunc {
	return func(c *echo.Context) error {
		schemaURL := html.EscapeString(fmt.Sprintf("%s/docs/schema.yaml", s.url))
		return c.HTML(http.StatusOK, fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>API Reference</title>
</head>
<body>
  <script id="api-reference" data-url="%s" data-configuration='%s'></script>
  <script src="%s"></script>
</body>
</html>`, schemaURL, scalarConfiguration, scalarBundleURL))
	}
}
