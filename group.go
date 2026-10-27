package ballo

import "strings"

// Group is a set of routes with shared middleware and prefix.
type Group struct {
	prefix      string
	middlewares []Middleware
	engine      *Engine
}

// GET registers a route on the group.
func (g *Group) GET(path string, handler Handler) {
	g.addRoute("GET", path, handler)
}

// POST registers a route on the group.
func (g *Group) POST(path string, handler Handler) {
	g.addRoute("POST", path, handler)
}

// PUT registers a route on the group.
func (g *Group) PUT(path string, handler Handler) {
	g.addRoute("PUT", path, handler)
}

// DELETE registers a route on the group.
func (g *Group) DELETE(path string, handler Handler) {
	g.addRoute("DELETE", path, handler)
}

// PATCH registers a route on the group.
func (g *Group) PATCH(path string, handler Handler) {
	g.addRoute("PATCH", path, handler)
}

// HEAD registers a route on the group.
func (g *Group) HEAD(path string, handler Handler) {
	g.addRoute("HEAD", path, handler)
}

// OPTIONS registers a route on the group.
func (g *Group) OPTIONS(path string, handler Handler) {
	g.addRoute("OPTIONS", path, handler)
}

// Use adds middleware to the group.
func (g *Group) Use(mw ...Middleware) {
	g.middlewares = append(g.middlewares, mw...)
}

// Route registers a route with the given method on the group.
func (g *Group) Route(method, path string, handler Handler) {
	g.addRoute(method, path, handler)
}

// Group creates a sub-group with inherited prefix and middleware.
func (g *Group) Group(prefix string, mw ...Middleware) *Group {
	p := strings.TrimSuffix(g.prefix, "/")
	r := strings.TrimPrefix(prefix, "/")
	sub := &Group{
		prefix:      p + "/" + r,
		middlewares: append([]Middleware{}, g.middlewares...),
		engine:      g.engine,
	}
	sub.middlewares = append(sub.middlewares, mw...)
	return sub
}

func (g *Group) addRoute(method, path string, handler Handler) {
	// Apply group middleware to handler
	if len(g.middlewares) > 0 {
		h := handler
		for i := len(g.middlewares) - 1; i >= 0; i-- {
			h = g.middlewares[i](h)
		}
		handler = h
	}
	// Clean prefix and path to avoid duplicate slashes
	p := strings.TrimSuffix(g.prefix, "/")
	r := strings.TrimPrefix(path, "/")
	fullPath := p + "/" + r
	g.engine.addRoute(method, fullPath, handler)
}
