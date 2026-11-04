package ballo

import (
	"net/http"
	"strings"
	"sync"

	bh "github.com/lukman-ss/ballo-http"
	ballorender "github.com/lukman-ss/ballo-render"
)

// Handler is an alias to ballo-http Handler.
type Handler = bh.Handler

// Middleware is an alias to ballo-http Middleware.
type Middleware = bh.Middleware

// Request is an alias to ballo-http Request.
type Request = bh.Request

// Response is an alias to ballo-http Response.
type Response = bh.Response

// Engine is the Ballo web framework core.
type Engine struct {
	mu               sync.RWMutex
	routes           *[8]*router // indexed by method bit
	middlewares      []Middleware
	notFound         Handler
	methodNotAllowed Handler
	panicHandler     func(http.ResponseWriter, *http.Request, interface{})
	renderer         ballorender.Renderer
}

const (
	methodGet     = 0
	methodPost    = 1
	methodPut     = 2
	methodDelete  = 3
	methodPatch   = 4
	methodHead    = 5
	methodOptions = 6
	methodAny     = 7
)

func methodBit(method string) int {
	switch strings.ToUpper(method) {
	case "GET":
		return methodGet
	case "POST":
		return methodPost
	case "PUT":
		return methodPut
	case "DELETE":
		return methodDelete
	case "PATCH":
		return methodPatch
	case "HEAD":
		return methodHead
	case "OPTIONS":
		return methodOptions
	default:
		return -1
	}
}

// New creates a new Ballo engine.
func New() *Engine {
	e := &Engine{
		routes:       newRoutes(),
		panicHandler: defaultPanicHandler,
	}
	return e
}

func newRoutes() *[8]*router {
	r := &[8]*router{}
	for i := range r {
		r[i] = newRouter()
	}
	return r
}

// Use adds global middleware.
func (e *Engine) Use(mw ...Middleware) {
	e.middlewares = append(e.middlewares, mw...)
}

// Group creates a route group with shared middleware.
func (e *Engine) Group(prefix string, mw ...Middleware) *Group {
	g := &Group{
		prefix: prefix,
		engine: e,
	}
	g.middlewares = append(g.middlewares, mw...)
	return g
}

// GET registers a route.
func (e *Engine) GET(path string, handler Handler) {
	e.addRoute("GET", path, handler)
}

// POST registers a route.
func (e *Engine) POST(path string, handler Handler) {
	e.addRoute("POST", path, handler)
}

// PUT registers a route.
func (e *Engine) PUT(path string, handler Handler) {
	e.addRoute("PUT", path, handler)
}

// DELETE registers a route.
func (e *Engine) DELETE(path string, handler Handler) {
	e.addRoute("DELETE", path, handler)
}

// PATCH registers a route.
func (e *Engine) PATCH(path string, handler Handler) {
	e.addRoute("PATCH", path, handler)
}

// HEAD registers a route.
func (e *Engine) HEAD(path string, handler Handler) {
	e.addRoute("HEAD", path, handler)
}

// OPTIONS registers a route.
func (e *Engine) OPTIONS(path string, handler Handler) {
	e.addRoute("OPTIONS", path, handler)
}

// Any registers a route for all methods.
func (e *Engine) Any(path string, handler Handler) {
	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		e.addRoute(m, path, handler)
	}
}

func (e *Engine) addRoute(method, path string, handler Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	bit := methodBit(method)
	if bit < 0 {
		return
	}
	e.routes[bit].add(method, path, handler)
}

// NotFound sets the 404 handler.
func (e *Engine) NotFound(h Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notFound = h
}

// MethodNotAllowed sets the 405 handler.
func (e *Engine) MethodNotAllowed(h Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.methodNotAllowed = h
}

// SetPanicHandler sets the panic recovery handler.
func (e *Engine) SetPanicHandler(h func(http.ResponseWriter, *http.Request, interface{})) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.panicHandler = h
}

// SetRenderer configures an optional view renderer on the engine.
func (e *Engine) SetRenderer(r ballorender.Renderer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.renderer = r
}

// Render returns the configured renderer, if any.
func (e *Engine) Render() ballorender.Renderer {
	e.mu.RLock()
	r := e.renderer
	e.mu.RUnlock()
	return r
}

// ServeHTTP implements http.Handler.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			e.mu.RLock()
			ph := e.panicHandler
			e.mu.RUnlock()
			ph(w, r, rec)
		}
	}()

	path := r.URL.Path
	// Strip trailing slash for matching (but preserve root)
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	method := strings.ToUpper(r.Method)
	bit := methodBit(method)

	// Snapshot engine state and perform all route matching under the read
	// lock so concurrent registration (which takes the write lock) cannot
	// mutate the router trees mid-match. Handler execution happens after the
	// lock is released.
	e.mu.RLock()
	rts := e.routes
	nf := e.notFound
	mna := e.methodNotAllowed
	mws := e.middlewares

	// Try exact method match first
	var match *routeMatch
	if bit >= 0 && rts[bit] != nil {
		match = rts[bit].match(path)
	}

	// No exact method match. Path may exist with another method → 405.
	var allowed []string
	if match == nil && bit >= 0 {
		for i, rt := range rts {
			if rt == nil || i == bit {
				continue
			}
			if m := rt.match(path); m != nil {
				for mt := range rt.methods {
					allowed = append(allowed, mt)
				}
			}
		}
	}
	e.mu.RUnlock()

	if match != nil {
		req := bh.NewRequest(r)
		req = bh.WithParams(req, match.params)
		resp, err := applyMiddlewares(match.handler, mws, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp == nil {
			resp, _ = bh.Error(http.StatusInternalServerError, "internal server error")
		}
		resp.Write(w)
		return
	}

	if len(allowed) > 0 {
		if mna != nil {
			req := bh.NewRequest(r)
			req = bh.WithParams(req, nil)
			resp, err := applyMiddlewares(mna, mws, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if resp == nil {
				resp, _ = bh.Error(http.StatusMethodNotAllowed, "method not allowed")
			}
			resp.Write(w)
			return
		}
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Path not found → 404.
	if nf != nil {
		req := bh.NewRequest(r)
		req = bh.WithParams(req, nil)
		resp, err := applyMiddlewares(nf, mws, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp == nil {
			resp, _ = bh.Error(http.StatusNotFound, "not found")
		}
		resp.Write(w)
		return
	}
	http.NotFound(w, r)
}

func applyMiddlewares(handler Handler, mw []Middleware, req *Request) (*Response, error) {
	h := handler
	// Apply middleware in reverse order so first-registered is outermost
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h(req)
}

func defaultPanicHandler(w http.ResponseWriter, r *http.Request, rec interface{}) {
	// Don't leak stack traces to client
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// JSON helper for handlers.
// On marshal failure returns a 500 error response to avoid silently producing nil.
func JSON(status int, data interface{}) *Response {
	r, err := bh.JSON(status, data)
	if err != nil {
		r, _ = bh.Error(http.StatusInternalServerError, "internal server error")
		return r
	}
	return r
}

// Text helper for handlers.
func Text(status int, text string) *Response {
	r, _ := bh.Text(status, text)
	if r == nil {
		r, _ = bh.Error(http.StatusInternalServerError, "internal server error")
	}
	return r
}

// HTML helper for handlers.
func HTML(status int, html string) *Response {
	r, _ := bh.HTML(status, html)
	if r == nil {
		r, _ = bh.Error(http.StatusInternalServerError, "internal server error")
	}
	return r
}

// Shutdown gracefully shuts down the server.
// Deprecated: Engine does not manage a server instance. Use context.WithTimeout with http.Server.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Clear routes (optional - for tests)
	for i := range e.routes {
		e.routes[i] = newRouter()
	}
	e.middlewares = nil
}
