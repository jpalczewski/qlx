package app

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/handler"
	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// Server is the top-level HTTP handler that wires all domain handlers together.
type Server struct {
	handler http.Handler
}

// NewServer creates the composition root: services, handlers, routes, middleware.
func NewServer(s store.Store, pm *qlprint.PrinterManager, cm *qlprint.ConnectionManager) *Server {
	// Load translations
	translations := webutil.NewTranslations()
	if err := translations.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		panic(err)
	}

	// Create services
	inventory := service.NewInventoryService(s)
	bulk := service.NewBulkService(s)
	tags := service.NewTagService(s)
	search := service.NewSearchService(s)
	printers := service.NewPrinterService(s)
	templates := service.NewTemplateService(s)
	export := service.NewExportService(s)
	notes := service.NewNoteService(s)

	// Build responder with template rendering
	tmplMap := handler.LoadTemplates(handler.TemplateFuncs{
		ResolveTags: tags.ResolveTagIDs,
		GetTag:      tags.GetTag,
	})
	resp := handler.NewHTMLResponder(tmplMap, translations)

	// Create domain handlers
	containerHandler := handler.NewContainerHandler(inventory, templates, printers, pm, notes, tags, resp)

	registrars := []handler.RouteRegistrar{
		containerHandler,
		handler.NewItemHandler(inventory, templates, printers, pm, notes, tags, resp),
		handler.NewTagHandler(tags, inventory, resp),
		handler.NewBulkHandler(bulk),
		handler.NewSearchHandler(search, resp),
		handler.NewPrintHandler(pm, cm, inventory, printers, templates, tags, notes, resp),
		handler.NewTemplateHandler(templates, pm, resp),
		handler.NewExportHandler(export, inventory),
		handler.NewNoteHandler(notes, inventory, resp),
		handler.NewPartialsHandler(inventory, search, tags, resp),
		handler.NewStatsHandler(inventory, tags, resp),
		handler.NewSettingsHandler(resp),
		handler.NewI18nHandler(translations),
		handler.NewAdhocHandler(pm, printers, templates, resp),
		handler.NewBluetoothHandler(),
		handler.NewDebugHandler(pm, printers, resp),
	}

	mux := http.NewServeMux()

	// Static files
	staticFS, err := fs.Sub(embedded.Static, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /static/icons/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(r.PathValue("name"), ".svg")
		data, err := palette.SVG(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(data) //nolint:errcheck,gosec
	})

	// Root handler — container list
	mux.HandleFunc("GET /{$}", containerHandler.List)

	// Register all domain routes
	for _, reg := range registrars {
		reg.RegisterRoutes(mux)
	}

	return &Server{handler: webutil.LangMiddleware("pl")(webutil.LoggingMiddleware(mux))}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
