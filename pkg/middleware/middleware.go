package middleware

import "net/http"

// System manages an ordered stack of HTTP middleware.
type System interface {
	Use(mw func(http.Handler) http.Handler)
	Apply(handler http.Handler) http.Handler
}

type mw struct {
	stack []func(http.Handler) http.Handler
}

// New creates an empty middleware System.
func New() System {
	return &mw{
		stack: []func(http.Handler) http.Handler{},
	}
}

func (m *mw) Use(fn func(http.Handler) http.Handler) {
	m.stack = append(m.stack, fn)
}

func (m *mw) Apply(handler http.Handler) http.Handler {
	for i := len(m.stack) - 1; i >= 0; i-- {
		handler = m.stack[i](handler)
	}
	return handler
}
