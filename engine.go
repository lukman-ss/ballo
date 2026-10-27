package ballo

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Handler receives a Request and returns a Response or error.
type Handler func(*Request) (*Response, error)

// Middleware wraps a Handler. Return nil to short-circuit (Response written directly).
type Middleware func(Handler) Handler

// Request wraps http.Request with Ballo extensions.
type Request struct {
	*http.Request
	Context context.Context
	rctx    *RouteContext
}

// RouteContext holds matched route info.
type RouteContext struct {
	Method  string
	Path    string
	Params  map[string]string
	RawPath string
}

// Response is the Ballo response type.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	committed  bool
}

// Write sends the response to the writer.
func (r *Response) Write(w http.ResponseWriter) {
	if r.committed {
		return
	}
	if r.StatusCode == 0 {
		r.StatusCode = http.StatusOK
	}
	for k, vv := range r.Header {
		w.Header()[k] = vv
	}
	w.WriteHeader(r.StatusCode)
	if r.Body != nil {
		w.Write(r.Body)
	}
	r.committed = true
}

// Engine is the Ballo web framework core.
type Engine struct {
	mu                 sync.RWMutex
	routes             *[8]*router // indexed by method bit
	middlewares        []Middleware
	notFound           Handler
	methodNotAllowed   Handler
	panicHandler       func(http.ResponseWriter, *http.Request, interface{})
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
		routes:        newRoutes(),
		panicHandler:  defaultPanicHandler,
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
		prefix:      prefix,
		middlewares: append([]Middleware{}, e.middlewares...),
		engine:      e,
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

	e.mu.RLock()
	rts := e.routes
	nf := e.notFound
	mna := e.methodNotAllowed
	ph := e.panicHandler
	e.mu.RUnlock()

	path := r.URL.Path
	// Strip trailing slash for matching (but preserve root)
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	method := strings.ToUpper(r.Method)
	bit := methodBit(method)

	// Try exact method match first
	var match *routeMatch
	if bit >= 0 && rts[bit] != nil {
		match = rts[bit].match(path)
	}

	// Fallback to other methods
	if match == nil && bit >= 0 {
		for i, rt := range rts {
			if rt == nil || i == bit {
				continue
			}
			match = rt.match(path)
			if match != nil {
				break
			}
		}
	}

	if match == nil {
		// Check if any route exists for this path with different method
		if e.hasPath(rts, path) {
			if mna != nil {
				req := e.toRequest(r, path, "")
				resp, err := mna(req)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if resp != nil {
					resp.Write(w)
				}
				return
			}
			w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if nf != nil {
			req := e.toRequest(r, path, "")
			resp, err := applyMiddlewares(nf, e.middlewares, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if resp != nil {
				resp.Write(w)
			}
			return
		}
		http.NotFound(w, r)
		return
	}

	req := e.toRequest(r, path, match.method)
	resp, err := applyMiddlewares(match.handler, e.middlewares, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if resp != nil {
		resp.Write(w)
	}
	_ = ph // used via panic recovery
}

func applyMiddlewares(handler Handler, mw []Middleware, req *Request) (*Response, error) {
	h := handler
	// Apply middleware in reverse order so first-registered is outermost
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h(req)
}

func (e *Engine) toRequest(r *http.Request, path, method string) *Request {
	return &Request{
		Request: r,
		Context: r.Context(),
		rctx: &RouteContext{
			Method:  method,
			Path:    path,
			Params:  make(map[string]string),
			RawPath: r.URL.RawPath,
		},
	}
}

func (e *Engine) hasPath(rts *[8]*router, path string) bool {
	for _, rt := range rts {
		if rt != nil && rt.hasPath(path) {
			return true
		}
	}
	return false
}

func defaultPanicHandler(w http.ResponseWriter, r *http.Request, rec interface{}) {
	// Don't leak stack traces to client
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// Params returns route parameters from context.
func (r *Request) Params() map[string]string {
	if r.rctx == nil {
		return nil
	}
	return r.rctx.Params
}

// Param returns a single route parameter.
func (r *Request) Param(key string) string {
	if r.rctx == nil {
		return ""
	}
	return r.rctx.Params[key]
}

// JSON helper for handlers.
func JSON(status int, data interface{}) *Response {
	body, err := jsonMarshal(data)
	if err != nil {
		body = []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		status = http.StatusInternalServerError
	}
	return &Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}
}

// Text helper for handlers.
func Text(status int, text string) *Response {
	return &Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       []byte(text),
	}
}

// HTML helper for handlers.
func HTML(status int, html string) *Response {
	return &Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
		Body:       []byte(html),
	}
}

// WithContext returns a new Request with the given context.
func (r *Request) WithContext(ctx context.Context) *Request {
	r2 := *r
	r2.Request = r.Request.WithContext(ctx)
	r2.Context = ctx
	return &r2
}

// Shutdown gracefully shuts down the server.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Clear routes (optional - for tests)
	for i := range e.routes {
		e.routes[i] = newRouter()
	}
	e.middlewares = nil
}
