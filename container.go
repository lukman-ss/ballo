package ballo

import (
	"fmt"
	"sync"
)

// Container is a minimal dependency injection container.
// It supports factory bindings (Bind), singleton instances (Instance), and resolution (Make).
type Container struct {
	mu        sync.RWMutex
	bindings  map[string]func(*Container) (any, error)
	instances map[string]any
}

// NewContainer creates an empty Container.
func NewContainer() *Container {
	return &Container{
		bindings:  make(map[string]func(*Container) (any, error)),
		instances: make(map[string]any),
	}
}

// Bind registers a factory for key. Called each time Make is invoked unless overridden by Instance.
func (c *Container) Bind(key string, factory func(*Container) (any, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[key] = factory
}

// Instance registers a pre-built singleton value for key.
func (c *Container) Instance(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances[key] = value
}

// Make resolves a value for key. Singleton instances take precedence over factory bindings.
func (c *Container) Make(key string) (any, error) {
	c.mu.RLock()
	if v, ok := c.instances[key]; ok {
		c.mu.RUnlock()
		return v, nil
	}
	factory, ok := c.bindings[key]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("ballo: no binding for %q", key)
	}
	return factory(c)
}

// MustMake resolves key and panics on error.
func (c *Container) MustMake(key string) any {
	v, err := c.Make(key)
	if err != nil {
		panic(err)
	}
	return v
}

// Has reports whether key has a binding or instance.
func (c *Container) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, b := c.bindings[key]
	_, i := c.instances[key]
	return b || i
}
