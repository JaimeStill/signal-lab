package routes

import "net/http"

// Route binds an HTTP method and pattern to a handler.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}
