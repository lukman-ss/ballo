package ballo

import (
	"net/http"

	bh "github.com/lukman-ss/ballo-http"
)

// HTTPRequest is an alias for ballo-http's Request type.
type HTTPRequest = bh.Request

// HTTPResponse is an alias for ballo-http's Response type.
type HTTPResponse = bh.Response

// Adapt converts a ballo-http Handler into a standard http.HandlerFunc.
func AdaptHTTP(h bh.Handler) http.HandlerFunc {
	return bh.Adapt(h)
}
