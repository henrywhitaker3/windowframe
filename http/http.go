// Package httphttp
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
	"github.com/henrywhitaker3/windowframe/http/common"
	"github.com/henrywhitaker3/windowframe/http/handlers/docs"
	"github.com/henrywhitaker3/windowframe/http/validation"
	"github.com/henrywhitaker3/windowframe/log"
	"github.com/henrywhitaker3/windowframe/tracing"
	"github.com/henrywhitaker3/windowframe/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
)

type SpecMutator func(*openapi3.Reflector)

type ErrorHandler func(err error) (int, any, bool)

type HTTPOpts struct {
	Port int

	ServiceName string
	Version     string
	// The publically accessible url of the api. Used for swagger UI
	PublicURL string

	OpenapiEnabled bool

	SpecMutations []SpecMutator

	Logger log.Logger
}

type HTTP struct {
	e              *echo.Echo
	spec           *openapi3.Reflector
	logger         log.Logger
	port           int
	openapiEnabled bool

	handleErrors []ErrorHandler

	Validator *validation.Validator
}

func New(opts HTTPOpts) *HTTP {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	r := openapi3.Reflector{}
	r.Spec = &openapi3.Spec{Openapi: "3.0.3"}
	r.Spec.Info.WithTitle(opts.ServiceName).WithVersion(opts.Version)
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
		openapiEnabled: opts.OpenapiEnabled,
		logger:         opts.Logger,
		handleErrors:   []ErrorHandler{},
	}

	h.e.HTTPErrorHandler = h.handleError

	if opts.OpenapiEnabled {
		h.e.GET("/docs/*", docs.NewSwagger(opts.PublicURL).Handler())
	}

	return h
}

func (h *HTTP) Start(ctx context.Context) error {
	if h.openapiEnabled {
		schema, err := h.spec.Spec.MarshalYAML()
		if err != nil {
			return fmt.Errorf("could not marshal openapi spec: %w", err)
		}
		Register(h, docs.NewSchema(string(schema)))
	}
	h.logger.Info("starting http server", "port", h.port)
	if err := h.e.Start(fmt.Sprintf(":%d", h.port)); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func (h *HTTP) Stop(ctx context.Context) error {
	h.logger.Info("stopping http server")
	return h.e.Shutdown(ctx)
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.e.ServeHTTP(w, r)
}

func (h *HTTP) Routes() []*echo.Route {
	return h.e.Routes()
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

func Register[Req any, Resp any](h *HTTP, handler Handler[Req, Resp]) {
	var reg func(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route

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
				return func(c echo.Context) error {
					return next(c)
				}
			},
		}
	}

	reg(handler.Metadata().Path, wrapHandler(h.Validator, handler, h.logger), mw...)
	if handler.Metadata().GenerateSpec {
		if err := buildSchema(h, handler); err != nil {
			panic(fmt.Errorf("invalid openapi spec: %w", err))
		}
	}
}

func wrapHandler[Req any, Resp any](
	v *validation.Validator,
	h Handler[Req, Resp],
	logger log.Logger,
) echo.HandlerFunc {
	handler := h.Handler()
	return func(c echo.Context) error {
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
		if err := v.Validate(req); err != nil {
			return err
		}
		span.End()

		resp, err := handler(c, req)
		if err != nil {
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

	return h.spec.AddOperation(opctx)
}

var (
	echoParams = regexp.MustCompile(`:[A-Za-z0-9]+`)
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

func (h *HTTP) handleError(err error, c echo.Context) {
	if h.isHTTPError(err) {
		herr := err.(*echo.HTTPError)
		_ = c.JSON(herr.Code, herr)
		return
	}

	for _, handler := range h.handleErrors {
		if code, resp, ok := handler(err); ok {
			_ = c.JSON(code, resp)
			return
		}

	}

	switch true {
	case errors.Is(err, pgx.ErrNoRows):
		_ = c.JSON(http.StatusNotFound, NewError("not found"))
	case errors.Is(err, sql.ErrNoRows):
		_ = c.JSON(http.StatusNotFound, NewError("not found"))

	case errors.Is(err, common.ErrValidation):
		_ = c.JSON(http.StatusUnprocessableEntity, NewError(err.Error()))

	case errors.Is(err, common.ErrBadRequest):
		_ = c.JSON(http.StatusBadRequest, NewError(err.Error()))

	case errors.Is(err, common.ErrUnauth):
		_ = c.JSON(http.StatusUnauthorized, NewError(err.Error()))

	case errors.Is(err, common.ErrForbidden):
		_ = c.JSON(http.StatusForbidden, NewError("fobidden"))

	case errors.Is(err, common.ErrNotFound):
		_ = c.JSON(http.StatusNotFound, NewError("not found"))
	}

	validErr := &validation.ValidationError{}
	if ok := errors.As(err, &validErr); ok {
		_ = c.JSON(http.StatusUnprocessableEntity, validErr)
		return
	}

	pgErr, ok := h.asPgError(err)
	if ok {
		switch pgErr.Code {
		// Unique constraint violation
		case "23505":
			_ = c.JSON(
				http.StatusUnprocessableEntity,
				NewError("a record with the same details already exists"),
			)
			return
		}
	}

	h.logger.ErrorContext(c.Request().Context(), "unhandled error", "error", err)
	if hub := sentryecho.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	}
	h.e.DefaultHTTPErrorHandler(err, c)
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

func (h *HTTP) isHTTPError(err error) bool {
	switch err.(type) {
	case *echo.HTTPError:
		return true
	default:
		return false
	}
}

func (h *HTTP) asPgError(err error) (*pgconn.PgError, bool) {
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
}
