// Package http
package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/henrywhitaker3/windowframe/v2/duration"
	"github.com/henrywhitaker3/windowframe/v2/http/common"
	"github.com/henrywhitaker3/windowframe/v2/http/handlers/docs"
	"github.com/henrywhitaker3/windowframe/v2/http/validation"
	"github.com/henrywhitaker3/windowframe/v2/log"
	"github.com/henrywhitaker3/windowframe/v2/tracing"
	"github.com/henrywhitaker3/windowframe/v2/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
)

type SpecMutator func(*openapi3.Reflector)

type ErrorHandler func(err error) (int, any, bool)

type OpenapiOpts struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	PublicURL      string

	BearerAuth []struct {
		Enabled     bool
		Name        string
		Format      string
		Description string
	}
}

type HTTPOpts struct {
	Port int

	Openapi OpenapiOpts

	SpecMutations []SpecMutator

	Logger log.Logger
}

type HTTP struct {
	e              *echo.Echo
	spec           *openapi3.Reflector
	logger         log.Logger
	port           int
	openapiEnabled bool

	cancel context.CancelFunc
	done   chan struct{}

	handleErrors []ErrorHandler

	Validator *validation.Validator
}

func New(opts HTTPOpts) *HTTP {
	e := echo.New()

	r := openapi3.Reflector{}
	r.Spec = &openapi3.Spec{Openapi: "3.0.3"}
	r.Spec.Info.WithTitle(opts.Openapi.ServiceName).WithVersion(opts.Openapi.ServiceVersion)
	r.Spec.Servers = append(r.Spec.Servers, openapi3.Server{
		URL: opts.Openapi.PublicURL,
	})
	for _, b := range opts.Openapi.BearerAuth {
		r.Spec.SetHTTPBearerTokenSecurity(b.Name, b.Format, b.Description)
	}
	if opts.Logger == nil {
		opts.Logger = log.NullLogger{}
	}
	opts.SpecMutations = append(opts.SpecMutations, addSpecTypes)
	for _, m := range opts.SpecMutations {
		m(&r)
	}
	h := &HTTP{
		e:              e,
		spec:           &r,
		port:           opts.Port,
		Validator:      validation.New(),
		openapiEnabled: opts.Openapi.Enabled,
		logger:         opts.Logger,
		handleErrors:   []ErrorHandler{},
	}

	h.e.HTTPErrorHandler = h.handleError

	if opts.Openapi.Enabled {
		h.e.GET("/docs/*", docs.NewSwagger(opts.Openapi.PublicURL).Handler())
	}

	return h
}

func (h *HTTP) Start(ctx context.Context) error {
	if h.openapiEnabled {
		schema, err := h.spec.Spec.MarshalYAML()
		if err != nil {
			return fmt.Errorf("could not marshal openapi spec: %w", err)
		}
		h.Register(docs.NewSchema(string(schema)))
	}

	runCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.done = make(chan struct{})
	defer close(h.done)

	h.logger.Info("starting http server", "port", h.port)
	sc := echo.StartConfig{
		Address:    fmt.Sprintf(":%d", h.port),
		HideBanner: true,
		HidePort:   true,
	}
	if err := sc.Start(runCtx, h.e); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func (h *HTTP) Stop(ctx context.Context) error {
	h.logger.Info("stopping http server")
	if h.cancel == nil {
		return nil
	}
	h.cancel()
	select {
	case <-h.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.e.ServeHTTP(w, r)
}

func (h *HTTP) Routes() echo.Routes {
	return h.e.Router().Routes()
}

func (h *HTTP) HandleErrors(funcs ...ErrorHandler) {
	h.handleErrors = append(h.handleErrors, funcs...)
}

func (h *HTTP) Use(mw echo.MiddlewareFunc) {
	h.e.Use(mw)
}

type Handler[Req any, Resp any] interface {
	Handler() common.Handler[Req, Resp]
	Middleware() []echo.MiddlewareFunc
	Metadata() common.Metadata
}

func (h *HTTP) Register[Req, Resp any](handler Handler[Req, Resp]) {
	var reg func(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo

	switch handler.Metadata().Method {
	case http.MethodGet:
		reg = h.e.GET
	case http.MethodPost:
		reg = h.e.POST
	case http.MethodPatch:
		reg = h.e.PATCH
	case http.MethodDelete:
		reg = h.e.DELETE
	case http.MethodPut:
		reg = h.e.PUT
	case http.MethodHead:
		reg = h.e.HEAD
	case http.MethodOptions:
		reg = h.e.OPTIONS
	default:
		panic("invalid http method registered")
	}

	mw := handler.Middleware()
	if len(mw) == 0 {
		// Add a empty middleware so []... doesn't add a nil item
		mw = []echo.MiddlewareFunc{
			func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c *echo.Context) error {
					return next(c)
				}
			},
		}
	}

	reg(handler.Metadata().Path, wrapHandler(h, handler, h.logger), mw...)
	if handler.Metadata().GenerateSpec {
		if err := buildSchema(h, handler); err != nil {
			panic(fmt.Errorf("invalid openapi spec: %w", err))
		}
	}
}

func wrapHandler[Req any, Resp any](
	ht *HTTP,
	h Handler[Req, Resp],
	logger log.Logger,
) echo.HandlerFunc {
	handler := h.Handler()
	return func(c *echo.Context) error {
		_, span := tracing.NewSpan(c.Request().Context(), "BindRequest")
		defer span.End()

		var req Req
		if err := c.Bind(&req); err != nil {
			logger.Debug("failed to bind request", "error", err)
			return common.ErrBadRequest
		}
		span.End()

		_, span = tracing.NewSpan(c.Request().Context(), "ValidateRequest")
		defer span.End()
		validErr := ht.Validator.Validate(req)
		span.End()

		resp, err := handler(c, req)
		if validErr != nil || err != nil {
			useErr := err
			if validErr != nil {
				useErr = validErr
			}
			if code, resp, handled := ht.getErrorCodeAndResponse(useErr); handled {
				return c.JSON(code, resp)
			}
			return err
		}

		if resp == nil {
			return c.NoContent(h.Metadata().Code)
		}

		switch h.Metadata().Kind {
		case common.KindString:
			return c.String(h.Metadata().Code, fmt.Sprintf("%v", *resp))
		case common.KindJSON:
			fallthrough
		default:
			return c.JSON(h.Metadata().Code, *resp)
		}
	}
}

func buildSchema[Req any, Resp any](h *HTTP, handler Handler[Req, Resp]) error {
	opctx, err := h.spec.NewOperationContext(
		handler.Metadata().Method,
		replaceParams(handler.Metadata().Path),
	)
	if err != nil {
		return err
	}
	var req Req
	opctx.AddReqStructure(req)
	var resp Resp
	opctx.AddRespStructure(
		resp,
		openapi.WithHTTPStatus(handler.Metadata().Code),
	)
	opctx.SetTags(handler.Metadata().Tag)
	opctx.SetSummary(handler.Metadata().Name)
	if handler.Metadata().Description != "" {
		opctx.SetDescription(handler.Metadata().Description)
	}
	opctx.AddRespStructure(
		map[string]string{},
		openapi.WithHTTPStatus(http.StatusUnprocessableEntity),
	)
	opctx.AddRespStructure(
		NewError("not found"),
		openapi.WithHTTPStatus(http.StatusNotFound),
	)
	opctx.AddRespStructure(
		NewError("unauthorised"),
		openapi.WithHTTPStatus(http.StatusUnauthorized),
	)
	opctx.AddRespStructure(
		NewError("forbidden"),
		openapi.WithHTTPStatus(http.StatusForbidden),
	)
	if handler.Metadata().Auth.Enabled {
		opctx.AddSecurity(handler.Metadata().Auth.Name, handler.Metadata().Auth.Scopes...)
	}

	return h.spec.AddOperation(opctx)
}

var (
	echoParams = regexp.MustCompile(`:[A-Za-z0-9-_]+`)
)

func replaceParams(path string) string {
	matches := echoParams.FindAllString(path, -1)
	for _, match := range matches {
		path = strings.ReplaceAll(
			path,
			match,
			fmt.Sprintf("{%s}", strings.ReplaceAll(match, ":", "")),
		)
	}
	return path
}

var defaultHTTPErrorHandler = echo.DefaultHTTPErrorHandler(false)

func (h *HTTP) handleError(c *echo.Context, err error) {
	if err == nil {
		return
	}
	if resp, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && resp.Committed {
		return
	}

	if !isHTTPError(err) {
		h.logger.ErrorContext(c.Request().Context(), "unhandled error", "error", err)
		if hub := sentryecho.GetHubFromContext(c); hub != nil && err != nil {
			hub.CaptureException(err)
		}
	}
	defaultHTTPErrorHandler(c, err)
}

func (h *HTTP) getErrorCodeAndResponse(err error) (int, any, bool) {
	if isHTTPError(err) {
		herr := err.(*echo.HTTPError)
		return herr.Code, herr, true
	}

	for _, handler := range h.handleErrors {
		if code, resp, ok := handler(err); ok {
			return code, resp, true
		}

	}

	switch true {
	case errors.Is(err, pgx.ErrNoRows):
		return http.StatusNotFound, NewError("not found"), true
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound, NewError("not found"), true

	case errors.Is(err, common.ErrValidation):
		return http.StatusUnprocessableEntity, NewError(err.Error()), true

	case errors.Is(err, common.ErrBadRequest):
		return http.StatusBadRequest, NewError(err.Error()), true

	case errors.Is(err, common.ErrUnauth):
		return http.StatusUnauthorized, NewError(err.Error()), true

	case errors.Is(err, common.ErrForbidden):
		return http.StatusForbidden, NewError("fobidden"), true

	case errors.Is(err, common.ErrNotFound):
		return http.StatusNotFound, NewError("not found"), true
	}

	validErr := &validation.ValidationError{}
	if ok := errors.As(err, &validErr); ok {
		return http.StatusUnprocessableEntity, validErr, true
	}

	pgErr, ok := asPgError(err)
	if ok {
		switch pgErr.Code {
		// Unique constraint violation
		case "23505":
			return http.StatusUnprocessableEntity, NewError(
				"a record with the same details already exists",
			), true
		}
	}

	return http.StatusServiceUnavailable, nil, false
}

type ErrorJSON struct {
	Message string `json:"message"`
}

func NewError(msg string) ErrorJSON {
	return ErrorJSON{Message: msg}
}

func (e ErrorJSON) Error() string {
	return e.Message
}

func isHTTPError(err error) bool {
	switch err.(type) {
	case *echo.HTTPError:
		return true
	default:
		return false
	}
}

func asPgError(err error) (*pgconn.PgError, bool) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg, true
	}
	return nil, false
}

func addSpecTypes(r *openapi3.Reflector) {
	uuidDef := jsonschema.Schema{}
	uuidDef.AddType(jsonschema.String)
	uuidDef.WithFormat("uuid")
	uuidDef.WithExamples("01972d8a-8038-7523-abb5-48a2bc60bedc")
	uuidDef.WithTitle("UUID")
	r.AddTypeMapping(uuid.UUID{}, uuidDef)

	strdurDef := jsonschema.Schema{}
	strdurDef.AddType(jsonschema.String)
	strdurDef.WithExamples("1s", "1m", "1h", "1d", "23h4m2s")
	strdurDef.WithTitle("String duration")
	var eg duration.StringDuration
	r.AddTypeMapping(eg, strdurDef)
}
