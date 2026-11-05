package ballo

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Container tests ---

func TestContainerBind(t *testing.T) {
	c := NewContainer()
	c.Bind("db", func(_ *Container) (any, error) {
		return "sqlite", nil
	})
	v, err := c.Make("db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "sqlite" {
		t.Fatalf("want sqlite, got %v", v)
	}
}

func TestContainerInstance(t *testing.T) {
	c := NewContainer()
	c.Instance("logger", "mylogger")
	v, err := c.Make("logger")
	if err != nil {
		t.Fatal(err)
	}
	if v != "mylogger" {
		t.Fatalf("want mylogger, got %v", v)
	}
}

func TestContainerInstanceOverridesBind(t *testing.T) {
	c := NewContainer()
	c.Bind("svc", func(_ *Container) (any, error) { return "from-bind", nil })
	c.Instance("svc", "from-instance")
	v, _ := c.Make("svc")
	if v != "from-instance" {
		t.Fatalf("want from-instance, got %v", v)
	}
}

func TestContainerMissingKey(t *testing.T) {
	c := NewContainer()
	_, err := c.Make("missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestContainerMustMakePanicsOnMissing(t *testing.T) {
	c := NewContainer()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	c.MustMake("not-there")
}

func TestContainerHas(t *testing.T) {
	c := NewContainer()
	if c.Has("x") {
		t.Fatal("should not have x")
	}
	c.Bind("x", func(_ *Container) (any, error) { return 1, nil })
	if !c.Has("x") {
		t.Fatal("should have x after Bind")
	}
	c2 := NewContainer()
	c2.Instance("y", 2)
	if !c2.Has("y") {
		t.Fatal("should have y after Instance")
	}
}

func TestContainerDependencyResolution(t *testing.T) {
	c := NewContainer()
	c.Instance("config", map[string]string{"dsn": "file::memory:"})
	c.Bind("repo", func(c *Container) (any, error) {
		cfg, err := c.Make("config")
		if err != nil {
			return nil, err
		}
		return "repo@" + cfg.(map[string]string)["dsn"], nil
	})
	v, err := c.Make("repo")
	if err != nil {
		t.Fatal(err)
	}
	if v != "repo@file::memory:" {
		t.Fatalf("unexpected: %v", v)
	}
}

// --- ServiceProvider / lifecycle tests ---

type recordProvider struct {
	name       string
	log        *[]string
	registerFn func(*App)
	bootFn     func(*App)
}

func (p *recordProvider) Register(app *App) {
	*p.log = append(*p.log, p.name+":register")
	if p.registerFn != nil {
		p.registerFn(app)
	}
}

func (p *recordProvider) Boot(app *App) {
	*p.log = append(*p.log, p.name+":boot")
	if p.bootFn != nil {
		p.bootFn(app)
	}
}

func TestAppLifecycleOrder(t *testing.T) {
	log := []string{}
	app := NewApp()
	app.Register(&recordProvider{name: "auth", log: &log})
	app.Register(&recordProvider{name: "session", log: &log})
	app.Boot()

	want := []string{"auth:register", "session:register", "auth:boot", "session:boot"}
	if len(log) != len(want) {
		t.Fatalf("want %v, got %v", want, log)
	}
	for i, w := range want {
		if log[i] != w {
			t.Errorf("step %d: want %q got %q", i, w, log[i])
		}
	}
}

func TestAppBootIdempotent(t *testing.T) {
	log := []string{}
	app := NewApp()
	app.Register(&recordProvider{name: "a", log: &log})
	app.Boot()
	app.Boot() // second call must be a no-op
	bootCount := 0
	for _, e := range log {
		if e == "a:boot" {
			bootCount++
		}
	}
	if bootCount != 1 {
		t.Fatalf("Boot called %d times, want 1", bootCount)
	}
}

func TestAppRegisterAfterBootPanics(t *testing.T) {
	app := NewApp()
	app.Boot()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when Register called after Boot")
		}
	}()
	app.Register(&recordProvider{name: "late", log: new([]string)})
}

func TestAppProviderCanBindAndResolve(t *testing.T) {
	app := NewApp()
	app.Register(&recordProvider{
		name: "db",
		log:  new([]string),
		registerFn: func(a *App) {
			a.Container.Instance("db", "sqlite3")
		},
		bootFn: func(a *App) {
			v, err := a.Container.Make("db")
			if err != nil || v != "sqlite3" {
				panic("db not resolved in Boot")
			}
		},
	})
	// must not panic
	app.Boot()
}

func TestAppBootCanResolveAcrossProviders(t *testing.T) {
	// provider B resolves a service that provider A registered
	app := NewApp()
	var resolved string
	app.Register(&recordProvider{
		name:       "a",
		log:        new([]string),
		registerFn: func(a *App) { a.Container.Instance("greeting", "hello") },
	})
	app.Register(&recordProvider{
		name: "b",
		log:  new([]string),
		bootFn: func(a *App) {
			v, _ := a.Container.Make("greeting")
			resolved = v.(string)
		},
	})
	app.Boot()
	if resolved != "hello" {
		t.Fatalf("want hello, got %q", resolved)
	}
}

func TestAppServeAndShutdown(t *testing.T) {
	app := NewApp()
	app.GET("/ping", func(req *Request) (*Response, error) {
		return JSON(http.StatusOK, map[string]string{"ok": "true"}), nil
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	errc := make(chan error, 1)
	go func() { errc <- app.Serve(addr) }()

	// wait until server accepts
	var resp *http.Response
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err = http.Get("http://" + addr + "/ping")
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("server did not start: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case e := <-errc:
		if e != nil {
			t.Fatalf("serve returned unexpected error: %v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestAppShutdownWithNoServer(t *testing.T) {
	app := NewApp()
	// must not panic or error
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAppEngineIntegrationViaProvider(t *testing.T) {
	app := NewApp()
	app.Register(&recordProvider{
		name: "routes",
		log:  new([]string),
		bootFn: func(a *App) {
			a.GET("/hello", func(req *Request) (*Response, error) {
				return JSON(http.StatusOK, map[string]string{"msg": "world"}), nil
			})
		},
	})
	app.Boot()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/hello", nil)
	app.Engine.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
