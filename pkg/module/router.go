package module

import (
	"net/http"
	"strings"
)

// Router dispatches requests to mounted modules by path prefix,
// falling back to a native ServeMux for unmatched paths.
type Router struct {
	modules map[string]*Module
	native  *http.ServeMux
}

// NewRouter creates a Router with an empty module map and native fallback mux.
func NewRouter() *Router {
	return &Router{
		modules: make(map[string]*Module),
		native:  http.NewServeMux(),
	}
}

// HandleNative registers a handler on the native fallback mux.
func (r *Router) HandleNative(pattern string, handler http.HandlerFunc) {
	r.native.HandleFunc(pattern, handler)
}

// Mount registers a module to handle requests matching its prefix.
func (r *Router) Mount(m *Module) {
	r.modules[m.prefix] = m
}

// ServeHTTP dispatches to the matching module or falls back to the native mux.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := normalizePath(req)
	prefix := extractPrefix(path)

	if m, ok := r.modules[prefix]; ok {
		m.Serve(w, req)
		return
	}

	r.native.ServeHTTP(w, req)
}

func extractPrefix(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 {
		return "/" + parts[1]
	}
	return path
}

func normalizePath(req *http.Request) string {
	path := req.URL.Path
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
		req.URL.Path = path
	}
	return path
}
