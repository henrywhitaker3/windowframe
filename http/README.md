# HTTP

A typed HTTP server built on [echo v5](https://github.com/labstack/echo). Handlers
declare their request and response types as generics; the server binds and
validates the request, maps returned errors onto status codes, and generates an
OpenAPI 3.0 spec from the same type information.

## Usage

A handler is any type implementing `Handler[Req, Resp]` — three methods that
describe the work, its middleware, and its metadata:

```go
type PingRequest struct {
    Name string `param:"name" json:"name" validate:"required"`
}

type PingResponse struct {
    Message string `json:"message"`
}

type Ping struct{}

func (p *Ping) Handler() common.Handler[PingRequest, PingResponse] {
    return func(c *echo.Context, req PingRequest) (*PingResponse, error) {
        return &PingResponse{Message: "hello " + req.Name}, nil
    }
}

func (p *Ping) Middleware() []echo.MiddlewareFunc {
    return []echo.MiddlewareFunc{}
}

func (p *Ping) Metadata() common.Metadata {
    return common.Metadata{
        Name:         "Ping",
        Description:  "Says hello",
        Tag:          "misc",
        Method:       http.MethodGet,
        Path:         "/ping/:name",
        Code:         http.StatusOK,
        GenerateSpec: true,
    }
}
```

Build the server, register handlers, and run it:

```go
srv := http.New(http.HTTPOpts{
    Port:   8080,
    Logger: slog.Default(),
    Openapi: http.OpenapiOpts{
        Enabled:        true,
        ServiceName:    "my-service",
        ServiceVersion: "1.2.3",
        PublicURL:      "https://api.example.com",
    },
})

srv.Use(middleware.Logger())
srv.Register(&Ping{})

// Blocks until ctx is cancelled
if err := srv.Start(ctx); err != nil {
    panic(err)
}
```

`Stop(ctx)` cancels the run context and waits for `Start` to return, or returns
`ctx.Err()` if the given context expires first. `*HTTP` also implements
`http.Handler`, so it can be driven directly by `httptest` without binding a
port.

## Request lifecycle

For every request, `Register` wraps the handler so that it:

1. Binds the request into `Req` via echo's binder (path, query, header, body —
   whatever the struct tags ask for). A bind failure returns `400` with
   `common.ErrBadRequest`.
2. Validates `Req` with the server's `*validation.Validator` (`validate` tags).
3. Calls the handler.
4. Writes the response: `nil` writes `Metadata().Code` with no content,
   `common.KindString` writes it as plain text, and anything else (including the
   zero value `common.KindJSON`) writes JSON.

Steps 2 and 3 both run before errors are considered: **the handler is invoked
even when validation failed**, and its response is discarded in favour of the
validation error. Handlers that must not run on invalid input should re-check
their input, or do the check in middleware.

## Errors

Handlers return ordinary errors; the server maps them to a status code and a
JSON body:

| Error | Status |
| --- | --- |
| `*echo.HTTPError` | its own code |
| `pgx.ErrNoRows`, `sql.ErrNoRows`, `common.ErrNotFound` | `404` |
| `common.ErrValidation`, `*validation.ValidationError` | `422` |
| Postgres unique violation (`23505`) | `422` |
| `common.ErrBadRequest` | `400` |
| `common.ErrUnauth` | `401` |
| `common.ErrForbidden` | `403` |

Anything unmatched falls through to echo's error handler as a `500`, is logged,
and is reported to Sentry when a hub is on the context (via
`sentryecho.GetHubFromContext`).

Register your own mappings with `HandleErrors`. They run after `*echo.HTTPError`
but before the built-ins, in registration order, and the first handler to return
`true` wins:

```go
srv.HandleErrors(func(err error) (int, any, bool) {
    if errors.Is(err, ErrQuotaExceeded) {
        return http.StatusPaymentRequired, http.NewError("quota exceeded"), true
    }
    return 0, nil, false
})
```

`http.NewError(msg)` builds the standard `{"message": "..."}` body.

## Validation

The validator is [go-playground/validator](https://github.com/go-playground/validator)
with `WithRequiredStructEnabled()` and field names taken from `json` tags.
Failures become a `*validation.ValidationError`, which marshals to a flat
`{"field": "tag"}` map. Add custom rules through the exported validator:

```go
srv.Validator.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
    return slugRegex.MatchString(fl.Field().String())
})
```

You can also build one by hand in middleware — `validation.Build().With("field",
"message")` returns an error that renders as `422`.

## OpenAPI

With `Openapi.Enabled`, the server serves Swagger UI at `/docs/*` and the spec
at `/docs/schema.yaml`. The spec is marshalled during `Start`, so **all handlers
must be registered before calling `Start`**.

Each handler with `GenerateSpec: true` contributes an operation built from `Req`
and `Resp`, plus stock `401`, `403`, `404` and `422` responses. Echo-style path
params are rewritten for the spec (`/user/:id` → `/user/{id}`). `uuid.UUID` and
`duration.StringDuration` are registered as string types with formats and
examples; add more via `SpecMutations`:

```go
srv := http.New(http.HTTPOpts{
    SpecMutations: []http.SpecMutator{
        func(r *openapi3.Reflector) {
            r.Spec.Info.WithDescription("Public API")
        },
    },
})
```

Bearer schemes are declared in `OpenapiOpts.BearerAuth` and attached to
operations through `Metadata().Auth`:

```go
Auth: common.Auth{Enabled: true, Name: "bearer", Scopes: []string{"users:read"}},
```

Note that this only documents the requirement — enforcing it is your
middleware's job.

## Middleware

Server-wide middleware goes through `srv.Use`; per-route middleware comes from
the handler's `Middleware()`. The `middleware` subpackage provides:

- **`Logger(mut ...LogAttributesFunc)`** — structured request logging through
  `slog`, including trace ID when one is present. It calls the error handler
  itself and then returns nil, so middleware registered before it (i.e. wrapping
  it) never sees the error. The variadic functions can decorate the logger with
  per-request attributes.
- **`Metrics(serviceName, registerer)`** — Prometheus request metrics, skipping
  `OPTIONS`.
- **`Tracing(serviceName)`** — OpenTelemetry spans, skipping `OPTIONS`,
  `/metrics` and `/`.
- **`Zap(level)`** — copies the request ID and trace ID onto the request context
  so the log package can pick them up.
- **`Idempotent(opts)`** — see below.

### Idempotency

`Idempotent` de-duplicates requests carrying an `X-Idempotency-Key` header (a
UUID). The first request takes a lock, runs, and has its status, headers and
body recorded; later requests with the same key replay that recorded response
without touching the handler. A request that arrives while the key is locked
gets `423 Locked`.

```go
store := middleware.NewIdempotentRueidisStore(rueidisClient)

srv.Use(middleware.Idempotent(middleware.IdempotentOpts{
    Store:           store,
    TTL:             24 * time.Hour, // how long a response is replayed for
    LockTTL:         time.Minute,    // lock expiry if the handler dies mid-flight
    SkipMissingKeys: middleware.Ptr(true),
}))
```

With `SkipMissingKeys` false, a request without a key is rejected as a
validation error. Supply any `IdempotencyStore` implementation to back it with
something other than Redis — `Get` must return `ErrIdempotentMissing` on a miss
and `Lock` must return `ErrIdempotentLocked` when the key is held.

## common

`http/common` holds the pieces handlers need without importing the server:

- `Handler[Req, Resp]`, `Metadata`, `Auth`, `KindJSON`/`KindString`.
- The sentinel errors above, plus `Wrap`/`Stack` from `pkg/errors`.
- Request context helpers: `RequestID`, `SetContextID`/`ContextID`,
  `SetTraceID`/`TraceID`, `SetRequest`/`GetRequest`,
  `SetAuthMethod`/`GetAuthMethod`.
- Token helpers: `GetToken` (bearer header falling back to the `user-auth`
  cookie), `GetRefreshToken`, `SetUserAuthCookie`, `SetUserRefreshTokenCookie`.
  The cookie setters take a domain or URL and write secure, http-only,
  `SameSite=None` cookies with a 30 day expiry.

## Behaviour to be aware of

- **`Register` panics** on an unsupported HTTP method, or when the handler's
  types produce an invalid OpenAPI operation. Both are startup-time programmer
  errors.
- **`Metadata().Code` is used for successes only**; error responses use the code
  from the mapping table.
- **`Metadata().Extras`** is a free-form `map[string]any` the server ignores —
  it's there for your own middleware to read.
- **`Stop` before `Start` is a no-op**, and returns nil.
