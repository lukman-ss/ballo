# Ballo

Ballo is a minimal, idiomatic Go web framework built on the standard library. It provides tree-based routing, route parameters, middleware chaining with short-circuit support, route groups, panic recovery, and structured JSON/Text/HTML responses.

## Features

- Tree router with route parameter and wildcard (`*`) support
- Middleware with consistent ordering (first registered = outermost) and short-circuit
- Route groups with shared prefix and middleware
- 404 / 405 handling with customizable handlers
- Panic recovery middleware
- JSON, Text, and HTML response helpers
- Thread-safe route registration (use before serving)

## Requirements

- Go 1.23+

## Installation

```bash
go get github.com/lukman-ss/ballo
```

## Quick Start

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

	e.GET("/health", func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.Text(http.StatusOK, "ok"), nil
	})

	// Custom error handlers
	e.NotFound(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusNotFound, map[string]string{"error": "not found"}), nil
	})
	e.MethodNotAllowed(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"}), nil
	})

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", e); err != nil {
		log.Fatal(err)
	}
}
```

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

## Lifecycle

Ballo's `Engine` implements `http.Handler`. Run it with the standard library:

```go
srv := &http.Server{
	Addr:    ":8080",
	Handler: e,
}
go srv.ListenAndServe()

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

Route and middleware registration should happen at startup, before `ListenAndServe`, since the engine is not guarded against concurrent registration during serving.

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

- Routes must be registered before serving begins. There is no hot-swapping during runtime.
- No built-in session, authentication, or ORM functionality.
- `Shutdown()` on the Engine clears routes (a test utility); use `http.Server.Shutdown(ctx)` for graceful server shutdown.
