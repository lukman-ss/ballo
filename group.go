package ballo

// Group is a set of routes with shared middleware and prefix.
type Group struct {
	prefix      string
	middlewares []Middleware
	engine      *Engine
}

// GET registers a route on the group.
func (g *Group) GET(path string, handler Handler) {
	g.engine.addRoute("GET", g.prefix+path, handler)
}

// POST registers a route on the group.
func (g *Group) POST(path string, handler Handler) {
	g.engine.addRoute("POST", g.prefix+path, handler)
}

// PUT registers a route on the group.
func (g *Group) PUT(path string, handler Handler) {
	g.engine.addRoute("PUT", g.prefix+path, handler)
}

// DELETE registers a route on the group.
func (g *Group) DELETE(path string, handler Handler) {
	g.engine.addRoute("DELETE", g.prefix+path, handler)
}

// PATCH registers a route on the group.
func (g *Group) PATCH(path string, handler Handler) {
	g.engine.addRoute("PATCH", g.prefix+path, handler)
}

// HEAD registers a route on the group.
func (g *Group) HEAD(path string, handler Handler) {
	g.engine.addRoute("HEAD", g.prefix+path, handler)
}

// OPTIONS registers a route on the group.
func (g *Group) OPTIONS(path string, handler Handler) {
	g.engine.addRoute("OPTIONS", g.prefix+path, handler)
}

// Use adds middleware to the group.
func (g *Group) Use(mw ...Middleware) {
	g.middlewares = append(g.middlewares, mw...)
}
