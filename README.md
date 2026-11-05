# Ballo

Ballo is a minimal, idiomatic Go web framework built on the standard library. It provides tree-based routing, route parameters, middleware chaining with short-circuit support, route groups, panic recovery, structured JSON/Text/HTML responses, an application lifecycle with service providers, and a service container for dependency injection.

## Features

- Tree router with route parameter and wildcard (`*`) support
- Middleware with consistent ordering (first registered = outermost) and short-circuit
- Route groups with shared prefix and middleware
- 404 / 405 handling with customizable handlers
- Panic recovery middleware
- JSON, Text, and HTML response helpers
- Thread-safe route registration (synchronized via internal RWMutex)
- Internal errors logged server-side; clients receive safe generic 500 response
- **Application lifecycle** — `NewApp → Register → Boot → Serve → Shutdown`
- **Service container** — `Bind`, `Instance`, `Make`, `Has`
- **Service provider hook** — consistent registration pattern for ballo-auth, ballo-session, ballo-render, ballo-migrate, and other packages

## Requirements

- Go 1.23+

## Installation

```bash
go get github.com/lukman-ss/ballo
```

## Quick Start (Engine only)

```go
package main

import (
	"log"
	"net/http"

	"github.com/lukman-ss/ballo"
)

func main() {
	e := ballo.New()
	e.Use(ballo.RecoveryMiddleware())

	e.GET("/hello/:name", func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusOK, map[string]string{
			"message": "Hello, " + req.Param("name"),
		}), nil
	})

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", e); err != nil {
		log.Fatal(err)
	}
}
```

## Application Lifecycle

`App` wraps the `Engine` and `Container`, and manages a registered list of `ServiceProvider`s.

```go
app := ballo.NewApp()

// Register providers — their Register() is called immediately in order.
app.Register(myAuthProvider)
app.Register(mySessionProvider)

// Boot all providers — calls Boot() on each in registration order.
// Serve() calls Boot() automatically if not already booted.
app.Boot()

// Add routes (can be done inside a provider's Boot method).
app.GET("/health", func(req *ballo.Request) (*ballo.Response, error) {
    return ballo.JSON(http.StatusOK, map[string]string{"status": "ok"}), nil
})

// Serve blocks until Shutdown or error.
go func() {
    if err := app.Serve(":8080"); err != nil {
        log.Fatal(err)
    }
}()

// Graceful shutdown.
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
app.Shutdown(ctx)
```

## Service Container

```go
// Factory binding — resolved fresh on each Make call.
app.Container.Bind("mailer", func(c *ballo.Container) (any, error) {
    cfg, _ := c.Make("config")
    return NewMailer(cfg.(*Config)), nil
})

// Singleton — registered value returned directly.
app.Container.Instance("config", &Config{DSN: "postgres://..."})

// Resolve.
v, err := app.Container.Make("mailer")
mailer := v.(*Mailer)

// Convenience; panics if key is missing.
mailer := app.Container.MustMake("mailer").(*Mailer)

// Check existence.
app.Container.Has("mailer") // true
```

## Service Providers

Implement `ballo.ServiceProvider` to integrate a package consistently:

```go
type AuthProvider struct{}

func (p *AuthProvider) Register(app *ballo.App) {
    // Bind services into the container. Do not resolve other services here.
    app.Container.Bind("auth", func(c *ballo.Container) (any, error) {
        return NewAuthService(), nil
    })
}

func (p *AuthProvider) Boot(app *ballo.App) {
    // All providers have been registered. Safe to resolve dependencies and register routes.
    app.GET("/login", loginHandler)
    app.GET("/logout", logoutHandler)
}
```

Register the provider before `Boot`:

```go
app.Register(&AuthProvider{})
```

Packages such as `ballo-auth`, `ballo-session`, `ballo-render`, and `ballo-migrate` can ship a `ServiceProvider` and be mounted with a single `app.Register(...)` call.

## Groups

Groups share a URL prefix and middleware. Middleware registered on a group is applied to all routes within it.

```go
api := e.Group("/api", authMiddleware)
api.GET("/users", listUsers)        // matches /api/users
api.GET("/users/:id", getUser)      // matches /api/users/:id
```

Sub-groups inherit parent middleware and prefix:

```go
v1 := api.Group("/v1")               // matches /api/v1/*
```

## Middleware

Middleware wraps handlers. First-registered middleware runs outermost. Return a non-nil `Response` (or error) without calling `next` to short-circuit.

```go
e.Use(func(next ballo.Handler) ballo.Handler {
	return func(req *ballo.Request) (*ballo.Response, error) {
		// before
		resp, err := next(req)
		// after
		return resp, err
	}
})
```

## Handlers and Responses

Handlers receive a `*Request` and return a `*Response` and `error`. The response is written exactly once.

```go
e.GET("/page", func(req *ballo.Request) (*ballo.Response, error) {
	return ballo.HTML(http.StatusOK, "<h1>Hi</h1>"), nil
})
```

Route parameters are accessed via `req.Param("name")` or `req.Params()`.

## Running Tests

```bash
go test -v -race ./...
```

## Example

A complete example application is in `example/main.go`. Run it with:

```bash
go run ./example
```

## Limitations

- Registering routes at startup (or inside `Boot`) is recommended over dynamic runtime mutations.
- No built-in session, authentication, or ORM functionality — use service providers.
- `Shutdown()` on the `Engine` clears routes (a test utility); use `App.Shutdown(ctx)` or `http.Server.Shutdown(ctx)` for graceful server shutdown.
