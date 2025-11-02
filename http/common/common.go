package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/henrywhitaker3/ctxgen"
	"github.com/labstack/echo/v4"
)

var (
	ctxIDKey   = "request_id"
	traceIDKey = "trace_id"

	UserAuthCookie   = "user-auth"
	UserRefreshToken = "user-refresh"
)

func RequestID(c echo.Context) string {
	return c.Response().Header().Get(echo.HeaderXRequestID)
}

func SetContextID(ctx context.Context, id string) context.Context {
	return ctxgen.WithValue(ctx, ctxIDKey, id)
}

func ContextID(ctx context.Context) string {
	return ctxgen.Value[string](ctx, ctxIDKey)
}

func SetTraceID(ctx context.Context, id string) context.Context {
	return ctxgen.WithValue(ctx, traceIDKey, id)
}

func TraceID(ctx context.Context) string {
	return ctxgen.Value[string](ctx, traceIDKey)
}

func SetRequest[T any](ctx context.Context, req T) context.Context {
	return ctxgen.WithValue(ctx, "request", req)
}

func GetRequest[T any](ctx context.Context) (T, bool) {
	return ctxgen.ValueOk[T](ctx, "request")
}

func GetToken(req *http.Request) string {
	header := req.Header.Get(echo.HeaderAuthorization)
	if header != "" {
		header = strings.Replace(header, "Bearer ", "", 1)
		return header
	}

	cookie, err := req.Cookie(UserAuthCookie)
	if err == nil {
		return cookie.Value
	}
	return ""
}

func GetRefreshToken(req *http.Request) (string, error) {
	cookie, err := req.Cookie(UserRefreshToken)
	if err != nil {
		return "", fmt.Errorf("get refresh token from cookie: %w", err)
	}
	return cookie.Value, nil
}

func SetUserAuthCookie(c echo.Context, domain string, token string) {
	c.SetCookie(&http.Cookie{
		Name:     UserAuthCookie,
		Value:    token,
		Path:     "/",
		Domain:   domain,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Now().Add(time.Minute * 5),
	})
}

func SetUserRefreshTokenCookie(c echo.Context, domain string, token string) {
	c.SetCookie(&http.Cookie{
		Name:     UserRefreshToken,
		Value:    token,
		Path:     "/",
		Domain:   domain,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Now().Add(time.Hour * 24 * 30),
	})
}
