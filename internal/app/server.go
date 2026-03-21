package app

import (
	"net/http"
	"strings"

	"github.com/erxyi/qlx/internal/api"
	"github.com/erxyi/qlx/internal/embedded"
	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/palette"
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

	inventory := service.NewInventoryService(s)
	bulk := service.NewBulkService(s)
	tags := service.NewTagService(s)
	search := service.NewSearchService(s)
	printers := service.NewPrinterService(s)

	uiServer := ui.NewServer(s, pm, translations, inventory, bulk, tags, search, printers)
	apiServer := api.NewServer(s, pm, translations, inventory, bulk, tags, search, printers)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(uiServer.StaticFS()))))
	mux.HandleFunc("GET /static/icons/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		name = strings.TrimSuffix(name, ".svg")
		data, err := palette.SVG(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(data) //nolint:errcheck
	})

	mux.HandleFunc("GET /", uiServer.HandleRoot)

	uiServer.RegisterRoutes(mux)
	apiServer.RegisterRoutes(mux)

	return &Server{handler: webutil.LangMiddleware("pl")(webutil.LoggingMiddleware(mux))}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
