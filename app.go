package ballo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// App is the Ballo application. It owns the Engine, Container, and ServiceProvider lifecycle.
//
// Lifecycle order:
//  1. NewApp()          — create application
//  2. app.Register(p)  — add one or more ServiceProviders (each provider's Register is called immediately)
//  3. app.Boot()       — calls Boot on every registered provider in registration order
//  4. app.Serve(addr)  — starts HTTP server (blocks until shutdown signal or error)
//  5. app.Shutdown(ctx)— graceful shutdown
type App struct {
	*Engine
	Container *Container

	mu        sync.Mutex
	providers []ServiceProvider
	booted    bool

	server *http.Server
}

// NewApp creates a new Ballo application with an empty Engine and Container.
func NewApp() *App {
	return &App{
		Engine:    New(),
		Container: NewContainer(),
	}
}

// Register adds a ServiceProvider to the application and immediately calls its Register method.
// Panics if Boot has already been called.
func (a *App) Register(p ServiceProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.booted {
		panic("ballo: Register called after Boot; all providers must be registered before Boot")
	}
	p.Register(a)
	a.providers = append(a.providers, p)
}

// Boot calls Boot on every registered provider in registration order.
// It is idempotent: subsequent calls are no-ops.
func (a *App) Boot() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.booted {
		return
	}
	for _, p := range a.providers {
		p.Boot(a)
	}
	a.booted = true
}

// Serve boots the application (if not already booted), then starts the HTTP server on addr.
// It blocks until the server stops. Use Shutdown to stop it gracefully.
func (a *App) Serve(addr string) error {
	a.Boot()

	srv := &http.Server{Addr: addr, Handler: a.Engine}
	a.mu.Lock()
	a.server = srv
	a.mu.Unlock()

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("ballo: serve %s: %w", addr, err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (a *App) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	srv := a.server
	a.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}
