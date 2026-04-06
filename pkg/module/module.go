package module

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/JaimeStill/signal-lab/pkg/middleware"
)

// Module is an HTTP handler that strips its prefix and delegates
// to an inner router with its own middleware stack.
type Module struct {
	prefix     string
	router     http.Handler
	middleware middleware.System
}

// New creates a Module with the given single-leve prefix (e.g. "/api").
// Panics if the prefix is empty, missing a leading slash, or multi-level.
func New(prefix string, router http.Handler) *Module {
	if err := validatePrefix(prefix); err != nil {
		panic(err)
	}
	return &Module{
		prefix:     prefix,
		router:     router,
		middleware: middleware.New(),
	}
}

// Handler returns the inner router wrapped with the module's middleware stack.
func (m *Module) Handler() http.Handler {
	return m.middleware.Apply(m.router)
}

// Prefix returns the module's path prefix
func (m *Module) Prefix() string {
	return m.prefix
}

// Serve strips the module prefix from the request path and dispatches the inner router.
func (m *Module) Serve(w http.ResponseWriter, req *http.Request) {
	path := extractPath(req.URL.Path, m.prefix)
	request := cloneRequest(req, path)
	m.Handler().ServeHTTP(w, request)
}

// Use adds middleware to the module's stack.
func (m *Module) Use(mw func(http.Handler) http.Handler) {
	m.middleware.Use(mw)
}

func cloneRequest(req *http.Request, path string) *http.Request {
	request := new(http.Request)
	*request = *req
	request.URL = new(url.URL)
	*request.URL = *req.URL
	request.URL.Path = path
	request.URL.RawPath = ""
	return request
}

func extractPath(fullPath, prefix string) string {
	path := fullPath[len(prefix):]
	if path == "" {
		return "/"
	}
	return path
}

func validatePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("module prefix cannot be empty")
	}
	if !strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("module prefix must start with /: %s", prefix)
	}
	if strings.Count(prefix, "/") != 1 {
		return fmt.Errorf("module prefix must be single-level sub-path: %s", prefix)
	}
	return nil
}
