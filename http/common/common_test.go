package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestItSetsUserAuthCookieForValidDomain(t *testing.T) {
	e := echo.New()

	tcs := []string{"example.org", "https://example.org", "http://example.org:8080"}
	for _, tc := range tcs {
		t.Run(tc, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := e.NewContext(&http.Request{}, rec)
			SetUserAuthCookie(c, tc, "token")
			require.Nil(t, c.NoContent(http.StatusOK))
			for _, c := range rec.Result().Cookies() {
				if c.Name == "user-auth" {
					require.Equal(t, "example.org", c.Domain)
					return
				}
			}
			require.True(t, false, "Did not match user-auth cookie")
		})
	}

}
