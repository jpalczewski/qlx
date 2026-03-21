package app

import (
	"net/http"

	"github.com/erxyi/qlx/internal/api"
	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
	"github.com/erxyi/qlx/internal/ui"
)

type Server struct {
	handler http.Handler
}

func NewServer(s *store.Store, ps *qlprint.PrintService) *Server {
	uiServer := ui.NewServer(s, ps)
	apiServer := api.NewServer(s, ps)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(uiServer.StaticFS()))))

	mux.HandleFunc("GET /", uiServer.HandleRoot)

	uiServer.RegisterRoutes(mux)
	apiServer.RegisterRoutes(mux)

	return &Server{handler: webutil.LoggingMiddleware(mux)}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
