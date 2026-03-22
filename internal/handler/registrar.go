package handler

import "net/http"

// RouteRegistrar registers HTTP routes on a mux.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}
