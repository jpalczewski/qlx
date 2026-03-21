package app

import (
	"net/http"

	"github.com/erxyi/qlx/internal/api"
	"github.com/erxyi/qlx/internal/embedded"
	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
	"github.com/erxyi/qlx/internal/ui"
)

type Server struct {
	handler http.Handler
}

func NewServer(s *store.Store, pm *qlprint.PrinterManager) *Server {
	translations := webutil.NewTranslations()
	if err := translations.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		panic(err)
	}

	uiServer := ui.NewServer(s, pm, translations)
	apiServer := api.NewServer(s, pm, translations)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(uiServer.StaticFS()))))

	mux.HandleFunc("GET /", uiServer.HandleRoot)

	uiServer.RegisterRoutes(mux)
	apiServer.RegisterRoutes(mux)

	return &Server{handler: webutil.LangMiddleware("pl")(webutil.LoggingMiddleware(mux))}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
