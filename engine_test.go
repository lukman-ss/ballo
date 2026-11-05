package ballo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEngineBasicRouting(t *testing.T) {
	e := New()
	e.GET("/hello", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, "world"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/hello", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "world" {
		t.Fatalf("expected 'world', got '%s'", w.Body.String())
	}
}

func TestEngineJSON(t *testing.T) {
	e := New()
	e.GET("/json", func(req *Request) (*Response, error) {
		return JSON(http.StatusOK, map[string]string{"foo": "bar"}), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/json", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected application/json*, got %s", w.Header().Get("Content-Type"))
	}

	var data map[string]string
	json.NewDecoder(w.Body).Decode(&data)
	if data["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %v", data)
	}
}

func TestEngineNotFound(t *testing.T) {
	e := New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/notfound", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestEngineCustomNotFound(t *testing.T) {
	e := New()
	e.NotFound(func(req *Request) (*Response, error) {
		return JSON(http.StatusNotFound, map[string]string{"error": "custom not found"}), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/notfound", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "custom not found") {
		t.Fatalf("expected custom not found body, got %s", w.Body.String())
	}
}

func TestEngineMethodNotAllowed(t *testing.T) {
	e := New()
	e.GET("/resource", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, "ok"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/resource", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestEngineRouteParams(t *testing.T) {
	e := New()
	e.GET("/users/:id", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, req.Param("id")), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/users/123", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "123" {
		t.Fatalf("expected '123', got '%s'", w.Body.String())
	}
}

func TestEngineMiddleware(t *testing.T) {
	e := New()
	var order []string

	mw1 := func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			order = append(order, "mw1-before")
			resp, err := next(req)
			order = append(order, "mw1-after")
			return resp, err
		}
	}

	mw2 := func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			order = append(order, "mw2-before")
			resp, err := next(req)
			order = append(order, "mw2-after")
			return resp, err
		}
	}

	e.Use(mw1, mw2)
	e.GET("/test", func(req *Request) (*Response, error) {
		order = append(order, "handler")
		return Text(http.StatusOK, "ok"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	e.ServeHTTP(w, r)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("step %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestEngineMiddlewareShortCircuit(t *testing.T) {
	e := New()
	var mwCalled bool
	var handlerCalled bool

	e.Use(func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			mwCalled = true
			return Text(http.StatusOK, "mw-response"), nil // short-circuit
		}
	})

	e.GET("/short", func(req *Request) (*Response, error) {
		handlerCalled = true
		return Text(http.StatusOK, "handler-response"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/short", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "mw-response" {
		t.Fatalf("expected 'mw-response', got '%s'", w.Body.String())
	}
	if !mwCalled {
		t.Fatal("middleware should have been called")
	}
	if handlerCalled {
		t.Fatal("handler should NOT have been called (short-circuit)")
	}
}

func TestEngineGroup(t *testing.T) {
	e := New()
	api := e.Group("/api")
	api.GET("/hello", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, "group hello"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/hello", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "group hello" {
		t.Fatalf("expected 'group hello', got '%s'", w.Body.String())
	}
}

func TestEnginePanicRecovery(t *testing.T) {
	e := New()
	e.GET("/panic", func(req *Request) (*Response, error) {
		panic("something went wrong")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/panic", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on panic, got %d", w.Code)
	}
}

func TestRecoveryMiddlewarePanic(t *testing.T) {
	e := New()
	e.Use(RecoveryMiddleware())
	e.GET("/panic-mw", func(req *Request) (*Response, error) {
		panic("middleware panic")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/panic-mw", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from RecoveryMiddleware, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "internal server error") {
		t.Fatalf("expected error body, got %s", w.Body.String())
	}
}

func TestEngineAllMethods(t *testing.T) {
	e := New()
	e.POST("/items", func(req *Request) (*Response, error) { return Text(201, "created"), nil })
	e.PUT("/items/1", func(req *Request) (*Response, error) { return Text(200, "updated"), nil })
	e.DELETE("/items/1", func(req *Request) (*Response, error) { return Text(204, ""), nil })
	e.PATCH("/items/1", func(req *Request) (*Response, error) { return Text(200, "patched"), nil })

	tests := []struct {
		method string
		path   string
		code   int
	}{
		{"POST", "/items", 201},
		{"PUT", "/items/1", 200},
		{"DELETE", "/items/1", 204},
		{"PATCH", "/items/1", 200},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tt.method, tt.path, nil)
		e.ServeHTTP(w, r)
		if w.Code != tt.code {
			t.Errorf("%s %s: expected %d, got %d", tt.method, tt.path, tt.code, w.Code)
		}
	}
}

func TestEngineGroupMiddleware(t *testing.T) {
	e := New()
	var mwCalled bool

	api := e.Group("/api", func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			mwCalled = true
			return next(req)
		}
	})

	api.GET("/test", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, "ok"), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/test", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !mwCalled {
		t.Fatal("group middleware was not called")
	}
}

func TestEngineWildcardRoute(t *testing.T) {
	e := New()
	e.GET("/static/*filepath", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, req.Param("filepath")), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/static/css/style.css", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "css/style.css" {
		t.Fatalf("expected 'css/style.css', got '%s'", w.Body.String())
	}
}

func TestEngineConcurrentRequests(t *testing.T) {
	e := New()
	e.GET("/users/:id", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, req.Param("id")), nil
	})

	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(id string) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/users/"+id, nil)
			e.ServeHTTP(w, r)
			if w.Code != http.StatusOK || w.Body.String() != id {
				t.Errorf("expected %s, got %s", id, w.Body.String())
			}
			done <- true
		}(string(rune('A' + (i % 26))))
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestEngineRenderer(t *testing.T) {
	e := New()
	if e.Render() != nil {
		t.Fatal("expected nil initial renderer")
	}

	mock := &mockRenderer{}
	e.SetRenderer(mock)
	if e.Render() != mock {
		t.Fatal("expected configured renderer")
	}
}

type mockRenderer struct{}

func (m *mockRenderer) Render(w io.Writer, name string, data any) error {
	_, err := io.WriteString(w, name)
	return err
}

func TestEngineJSONMarshalError(t *testing.T) {
	e := New()
	type circular struct {
		Ref *circular
	}
	c := &circular{}
	c.Ref = c
	e.GET("/badjson", func(req *Request) (*Response, error) {
		return JSON(http.StatusOK, c), nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/badjson", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestEngineNilResponseHandling(t *testing.T) {
	e := New()
	e.GET("/nil", func(req *Request) (*Response, error) {
		return nil, nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/nil", nil)
	e.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestEngineErrorDoesNotLeakDetails(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Engine)
		path   string
		method string
	}{
		{
			name: "handler error",
			setup: func(e *Engine) {
				e.GET("/error", func(req *Request) (*Response, error) {
					return nil, fmt.Errorf("internal database connection failed")
				})
			},
			path:   "/error",
			method: "GET",
		},
		{
			name: "404 handler error",
			setup: func(e *Engine) {
				e.NotFound(func(req *Request) (*Response, error) {
					return nil, fmt.Errorf("custom 404 internal error")
				})
			},
			path:   "/nonexistent",
			method: "GET",
		},
		{
			name: "405 handler error",
			setup: func(e *Engine) {
				e.GET("/resource", func(req *Request) (*Response, error) {
					return Text(http.StatusOK, "ok"), nil
				})
				e.MethodNotAllowed(func(req *Request) (*Response, error) {
					return nil, fmt.Errorf("method not allowed internal error")
				})
			},
			path:   "/resource",
			method: "POST",
		},
		{
			name: "middleware error",
			setup: func(e *Engine) {
				e.Use(func(next Handler) Handler {
					return func(req *Request) (*Response, error) {
						return nil, fmt.Errorf("middleware internal error")
					}
				})
				e.GET("/mw-error", func(req *Request) (*Response, error) {
					return Text(http.StatusOK, "ok"), nil
				})
			},
			path:   "/mw-error",
			method: "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New()
			tt.setup(e)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, tt.path, nil)
			e.ServeHTTP(w, r)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 500, got %d", w.Code)
			}
			body := w.Body.String()
			if strings.Contains(body, "database connection") || strings.Contains(body, "internal error") {
				t.Errorf("response body leaked internal error details: %s", body)
			}
			if !strings.Contains(body, "internal server error") {
				t.Errorf("expected generic error message, got: %s", body)
			}
		})
	}
}

func TestEngineConcurrentRegisterAndServe(t *testing.T) {
	e := New()
	e.GET("/safe", func(req *Request) (*Response, error) {
		return Text(http.StatusOK, "ok"), nil
	})
	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/safe", nil)
			e.ServeHTTP(w, r)
			time.Sleep(time.Millisecond)
		}
	}()

	time.Sleep(20 * time.Millisecond)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			path := fmt.Sprintf("/r%d", idx)
			e.GET(path, func(req *Request) (*Response, error) {
				return Text(http.StatusOK, "x"), nil
			})
		}()
		time.Sleep(time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
