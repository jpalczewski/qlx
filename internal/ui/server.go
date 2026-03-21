package ui

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type contextKey string

const langKey contextKey = "lang"

// Translations is a stub i18n registry. Keys and values will be populated in a future task.
type Translations struct {
	langs map[string]map[string]string
}

// NewTranslations returns an empty Translations with language buckets pre-initialized.
func NewTranslations() *Translations {
	return &Translations{langs: map[string]map[string]string{
		"pl": {},
		"en": {},
	}}
}

// Get returns the translation for key in lang, falling back to "en", then the key itself.
func (t *Translations) Get(lang, key string) string {
	if val, ok := t.langs[lang][key]; ok {
		return val
	}
	if val, ok := t.langs["en"][key]; ok {
		return val
	}
	return key
}

// PageData is the top-level template context for all page and partial renders.
// It carries the active language, a translation accessor, and the page-specific data.
type PageData struct {
	Lang       string
	translator *Translations
	Data       any
}

// T returns the translation for key in the active language.
func (p PageData) T(key string) string {
	return p.translator.Get(p.Lang, key)
}

type Server struct {
	store          *store.Store
	printerManager *print.PrinterManager
	templates      map[string]*template.Template
	staticFS       fs.FS
	translations   *Translations
}

type ContainerListData struct {
	Children  []store.Container
	Items     []store.Item
	Container *store.Container
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
}

type ItemDetailData struct {
	Item      *store.Item
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
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

type TagTreeData struct {
	Tags   []store.Tag
	Parent *store.Tag
	Path   []store.Tag
}

type SearchResultsData struct {
	Query      string
	Containers []store.Container
	Items      []store.Item
	Tags       []store.Tag
}

type TagChipsData struct {
	ObjectID   string
	ObjectType string
	Tags       []store.Tag
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
	layoutContent, err := embedded.Templates.ReadFile("templates/layouts/base.html")
	if err != nil {
		panic(err)
	}
	layoutTmpl := template.Must(template.New("layout").Funcs(template.FuncMap{
		"dict": dict,
		"resolveTags": func(ids []string) []store.Tag {
			var tags []store.Tag
			for _, id := range ids {
				if t := s.GetTag(id); t != nil {
					tags = append(tags, *t)
				}
			}
			return tags
		},
	}).Parse(string(layoutContent)))

	// Discover and parse all partials and components into the shared layout template.
	for _, dir := range []string{"templates/partials", "templates/components"} {
		err := fs.WalkDir(embedded.Templates, dir, func(fpath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || path.Ext(fpath) != ".html" {
				return nil
			}
			content, err := embedded.Templates.ReadFile(fpath)
			if err != nil {
				return err
			}
			layoutTmpl = template.Must(layoutTmpl.Parse(string(content)))
			return nil
		})
		if err != nil {
			panic(err)
		}
	}

	// Discover page templates and build the templates map.
	templates := make(map[string]*template.Template)
	err = fs.WalkDir(embedded.Templates, "templates/pages", func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(fpath) != ".html" {
			return nil
		}
		name := strings.ReplaceAll(strings.TrimSuffix(path.Base(fpath), ".html"), "_", "-")
		content, err := embedded.Templates.ReadFile(fpath)
		if err != nil {
			return err
		}
		tmpl, err := layoutTmpl.Clone()
		if err != nil {
			return err
		}
		tmpl = template.Must(tmpl.Parse(string(content)))
		wrapper := `{{ define "content" }}{{ template "` + name + `" . }}{{ end }}`
		tmpl = template.Must(tmpl.Parse(wrapper))
		templates[name] = tmpl
		return nil
	})
	if err != nil {
		panic(err)
	}

	staticFS, err := fs.Sub(embedded.Static, "static")
	if err != nil {
		panic(err)
	}

	return &Server{store: s, printerManager: pm, templates: templates, staticFS: staticFS, translations: NewTranslations()}
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
	mux.HandleFunc("POST /ui/actions/print-image", s.HandlePrintImage)
	mux.HandleFunc("POST /ui/actions/assets", s.HandleAssetUpload)
	mux.HandleFunc("GET /ui/actions/assets/{id}", s.HandleAssetServe)
	mux.HandleFunc("GET /ui/actions/containers/{id}/items-json", s.HandleContainerItemsJSON)

	mux.HandleFunc("GET /ui/templates", s.HandleTemplates)
	mux.HandleFunc("GET /ui/templates/new", s.HandleTemplateNew)
	mux.HandleFunc("GET /ui/templates/{id}/edit", s.HandleTemplateEdit)
	mux.HandleFunc("DELETE /ui/actions/templates/{id}", s.HandleTemplateDelete)
	mux.HandleFunc("POST /ui/actions/templates", s.HandleTemplateSave)
	mux.HandleFunc("PUT /ui/actions/templates/{id}", s.HandleTemplateSave)

	// Tags UI
	mux.HandleFunc("GET /ui/tags", s.HandleTags)
	mux.HandleFunc("POST /ui/actions/tags", s.HandleTagCreate)
	mux.HandleFunc("PUT /ui/actions/tags/{id}", s.HandleTagUpdate)
	mux.HandleFunc("DELETE /ui/actions/tags/{id}", s.HandleTagDelete)
	mux.HandleFunc("POST /ui/actions/tags/{id}/move", s.HandleTagMove)

	// Tag assignment
	mux.HandleFunc("POST /ui/actions/items/{id}/tags", s.HandleItemTagAdd)
	mux.HandleFunc("DELETE /ui/actions/items/{id}/tags/{tag_id}", s.HandleItemTagRemove)
	mux.HandleFunc("POST /ui/actions/containers/{id}/tags", s.HandleContainerTagAdd)
	mux.HandleFunc("DELETE /ui/actions/containers/{id}/tags/{tag_id}", s.HandleContainerTagRemove)

	// Partials
	mux.HandleFunc("GET /ui/partials/tree", s.HandleTreePartial)
	mux.HandleFunc("GET /ui/partials/tree/search", s.HandleTreeSearchPartial)
	mux.HandleFunc("GET /ui/partials/tag-tree", s.HandleTagTreePartial)
	mux.HandleFunc("GET /ui/partials/tag-tree/search", s.HandleTagTreeSearchPartial)

	// Bulk
	mux.HandleFunc("POST /ui/actions/bulk/move", s.HandleBulkMove)
	mux.HandleFunc("POST /ui/actions/bulk/delete", s.HandleBulkDelete)
	mux.HandleFunc("POST /ui/actions/bulk/tags", s.HandleBulkTags)

	// Search
	mux.HandleFunc("GET /ui/search", s.HandleSearch)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := "pl"
	if v := r.Context().Value(langKey); v != nil {
		lang = v.(string)
	}

	page := PageData{
		Lang:       lang,
		translator: s.translations,
		Data:       data,
	}

	templateName := "layout"
	if webutil.IsHTMX(r) {
		templateName = name
	}

	if err := tmpl.ExecuteTemplate(w, templateName, page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderPartial executes a named template directly without the layout wrapper.
// Use this for HTMX partial responses (fragments, not full pages).
func (s *Server) renderPartial(w http.ResponseWriter, r *http.Request, tmplName, defineName string, data any) {
	tmpl, ok := s.templates[tmplName]
	if !ok {
		http.Error(w, "template not found: "+tmplName, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := "pl"
	if v := r.Context().Value(langKey); v != nil {
		lang = v.(string)
	}

	page := PageData{
		Lang:       lang,
		translator: s.translations,
		Data:       data,
	}

	if err := tmpl.ExecuteTemplate(w, defineName, page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) containerViewModel(containerID string) (ContainerListData, bool) {
	printers := s.store.AllPrinters()
	templates := s.store.AllTemplates()

	if containerID == "" {
		return ContainerListData{
			Children:  s.store.ContainerChildren(""),
			Printers:  printers,
			Templates: templates,
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
		Printers:  printers,
		Templates: templates,
	}, true
}

func (s *Server) itemDetailViewModel(itemID string) (ItemDetailData, bool) {
	item := s.store.GetItem(itemID)
	if item == nil {
		return ItemDetailData{}, false
	}

	return ItemDetailData{
		Item:      item,
		Path:      s.store.ContainerPath(item.ContainerID),
		Printers:  s.store.AllPrinters(),
		Templates: s.store.AllTemplates(),
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
