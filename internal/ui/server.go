package ui

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type Server struct {
	store          *store.Store
	printerManager *print.PrinterManager
	templates      map[string]*template.Template
	staticFS       fs.FS
}

type ContainerListData struct {
	Children  []store.Container
	Items     []store.Item
	Container *store.Container
	Path      []store.Container
}

type ItemDetailData struct {
	Item     *store.Item
	Path     []store.Container
	Printers []store.PrinterConfig
}

type PrintersData struct {
	Printers []store.PrinterConfig
	Encoders []EncoderData
}

type EncoderData struct {
	Name   string
	Models []encoder.ModelInfo
}

type TemplateListData struct {
	Templates []store.Template
	Tags      []string
	ActiveTag string
}

type DesignerData struct {
	TemplateID        string
	TemplateName      string
	TemplateTags      string
	Target            string
	Width             float64
	Height            float64
	TemplateJSON      string
	PrinterModels     []encoder.ModelInfo
	PrinterModelsJSON string
	PreviewDataJSON   string
}

func NewServer(s *store.Store, pm *print.PrinterManager) *Server {
	layoutContent, err := embedded.Templates.ReadFile("templates/layout.html")
	if err != nil {
		panic(err)
	}
	layoutTmpl := template.Must(template.New("layout").Funcs(template.FuncMap{
		"dict": dict,
	}).Parse(string(layoutContent)))

	sharedFiles := []string{
		"templates/partials/breadcrumb.html",
		"templates/components/form_fields.html",
	}
	for _, path := range sharedFiles {
		content, err := embedded.Templates.ReadFile(path)
		if err != nil {
			panic(err)
		}
		layoutTmpl = template.Must(layoutTmpl.Parse(string(content)))
	}

	templateFiles := map[string]string{
		"containers":     "templates/containers.html",
		"item":           "templates/item.html",
		"item-form":      "templates/item_form.html",
		"container-form": "templates/container_form.html",
		"printers":       "templates/printers.html",
		"templates":          "templates/templates.html",
		"template-designer": "templates/template_designer.html",
	}

	templates := make(map[string]*template.Template)
	for name, path := range templateFiles {
		content, err := embedded.Templates.ReadFile(path)
		if err != nil {
			panic(err)
		}
		tmpl, err := layoutTmpl.Clone()
		if err != nil {
			panic(err)
		}
		tmpl = template.Must(tmpl.Parse(string(content)))
		wrapper := `{{ define "content" }}{{ template "` + name + `" . }}{{ end }}`
		tmpl = template.Must(tmpl.Parse(wrapper))
		templates[name] = tmpl
	}

	staticFS, err := fs.Sub(embedded.Static, "static")
	if err != nil {
		panic(err)
	}

	return &Server{store: s, printerManager: pm, templates: templates, staticFS: staticFS}
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}

	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		result[key] = values[i+1]
	}

	return result, nil
}

func (s *Server) StaticFS() fs.FS {
	return s.staticFS
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ui", s.HandleRoot)
	mux.HandleFunc("GET /ui/containers/{id}", s.HandleContainer)
	mux.HandleFunc("GET /ui/items/{id}", s.HandleItem)

	mux.HandleFunc("POST /ui/actions/containers", s.HandleContainerCreate)
	mux.HandleFunc("PUT /ui/actions/containers/{id}", s.HandleContainerUpdate)
	mux.HandleFunc("DELETE /ui/actions/containers/{id}", s.HandleContainerDelete)
	mux.HandleFunc("POST /ui/actions/items", s.HandleItemCreate)
	mux.HandleFunc("PUT /ui/actions/items/{id}", s.HandleItemUpdate)
	mux.HandleFunc("DELETE /ui/actions/items/{id}", s.HandleItemDelete)
	mux.HandleFunc("POST /ui/actions/containers/{id}/move", s.HandleContainerMove)
	mux.HandleFunc("POST /ui/actions/items/{id}/move", s.HandleItemMove)

	mux.HandleFunc("GET /ui/printers", s.HandlePrinters)
	mux.HandleFunc("POST /ui/actions/printers", s.HandlePrinterCreate)
	mux.HandleFunc("DELETE /ui/actions/printers/{id}", s.HandlePrinterDelete)
	mux.HandleFunc("POST /ui/actions/items/{id}/print", s.HandleItemPrint)

	mux.HandleFunc("GET /ui/templates", s.HandleTemplates)
	mux.HandleFunc("GET /ui/templates/new", s.HandleTemplateNew)
	mux.HandleFunc("GET /ui/templates/{id}/edit", s.HandleTemplateEdit)
	mux.HandleFunc("DELETE /ui/actions/templates/{id}", s.HandleTemplateDelete)
	mux.HandleFunc("POST /ui/actions/templates", s.HandleTemplateSave)
	mux.HandleFunc("PUT /ui/actions/templates/{id}", s.HandleTemplateSave)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	templateName := "layout"
	if webutil.IsHTMX(r) {
		templateName = name
	}

	if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) containerViewModel(containerID string) (ContainerListData, bool) {
	if containerID == "" {
		return ContainerListData{
			Children: s.store.ContainerChildren(""),
		}, true
	}

	container := s.store.GetContainer(containerID)
	if container == nil {
		return ContainerListData{}, false
	}

	return ContainerListData{
		Children:  s.store.ContainerChildren(containerID),
		Items:     s.store.ContainerItems(containerID),
		Container: container,
		Path:      s.store.ContainerPath(containerID),
	}, true
}

func (s *Server) itemDetailViewModel(itemID string) (ItemDetailData, bool) {
	item := s.store.GetItem(itemID)
	if item == nil {
		return ItemDetailData{}, false
	}

	return ItemDetailData{
		Item:     item,
		Path:     s.store.ContainerPath(item.ContainerID),
		Printers: s.store.AllPrinters(),
	}, true
}

func (s *Server) printersViewModel() PrintersData {
	printers := s.store.AllPrinters()
	var encoders []EncoderData
	for name, enc := range s.printerManager.AvailableEncoders() {
		encoders = append(encoders, EncoderData{
			Name:   name,
			Models: enc.Models(),
		})
	}
	return PrintersData{
		Printers: printers,
		Encoders: encoders,
	}
}
