package docs

import (
	"fmt"

	"github.com/labstack/echo/v4"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type SwaggerHandler struct {
	url string
}

func NewSwagger(url string) *SwaggerHandler {
	return &SwaggerHandler{
		url: url,
	}
}

func (s *SwaggerHandler) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		httpSwagger.Handler(
			httpSwagger.URL(fmt.Sprintf("%s/docs/schema.yaml", s.url)),
		)(
			c.Response().Writer,
			c.Request(),
		)
		return nil
	}
}
