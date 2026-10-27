package ballo

import (
	"log"
	"net/http"
	"time"
)

// LoggerMiddleware logs request method, path, status, and duration.
func LoggerMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			start := time.Now()
			resp, err := next(req)
			duration := time.Since(start)
			status := http.StatusOK
			if resp != nil {
				status = resp.StatusCode
			}
			log.Printf("%s %s %d %v", req.Method, req.URL.Path, status, duration)
			return resp, err
		}
	}
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("panic recovered: %v", rec)
				}
			}()
			return next(req)
		}
	}
}

// CORSMiddleware adds CORS headers.
func CORSMiddleware(allowedOrigins ...string) Middleware {
	origin := "*"
	if len(allowedOrigins) > 0 {
		origin = allowedOrigins[0]
	}
	return func(next Handler) Handler {
		return func(req *Request) (*Response, error) {
			resp, err := next(req)
			if resp == nil {
				resp = &Response{}
			}
			if resp.Header == nil {
				resp.Header = make(http.Header)
			}
			resp.Header.Set("Access-Control-Allow-Origin", origin)
			resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			return resp, err
		}
	}
}

// Static next middleware - does nothing.
var Next = func(next Handler) Handler {
	return next
}
