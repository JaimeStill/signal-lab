package routes

import "net/http"

// Group organizes routes under a common prefix.
type Group struct {
	Prefix   string
	Routes   []Route
	Children []Group
}

// Register adds all routes from the given groups to the mux.
func Register(mux *http.ServeMux, groups ...Group) {
	for _, group := range groups {
		registerGroup(mux, "", group)
	}
}

func registerGroup(mux *http.ServeMux, parentPrefix string, group Group) {
	fullPrefix := parentPrefix + group.Prefix
	for _, route := range group.Routes {
		pattern := route.Method + " " + fullPrefix + route.Pattern
		mux.HandleFunc(pattern, route.Handler)
	}
	for _, child := range group.Children {
		registerGroup(mux, fullPrefix, child)
	}
}
