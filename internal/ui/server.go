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
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// PageData is the top-level template context for all page and partial renders.
// It carries the active language, a translation accessor, and the page-specific data.
type PageData struct {
	Lang       string
	translator *webutil.Translations
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
	translations   *webutil.Translations
	inventory      *service.InventoryService
	bulk           *service.BulkService
	tags           *service.TagService
	search         *service.SearchService
	printers       *service.PrinterService
}

type ContainerListData struct {
	Children  []store.Container
	Items     []store.Item
	Container *store.Container
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
	Schemas   []string
}

type ItemDetailData struct {
	Item      *store.Item
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
	Schemas   []string
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
	Tags         []store.Tag
	Parent       *store.Tag
	Path         []store.Tag
	DefaultColor string
	DefaultIcon  string
}

// ContainerFormData is the view model for the container create/edit form.
type ContainerFormData struct {
	Container *store.Container
	Path      []store.Container
	ParentID  string
}

// ItemFormData is the view model for the item create/edit form.
type ItemFormData struct {
	Item        *store.Item
	Path        []store.Container
	ContainerID string
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

// TagStats holds statistics for a tag detail page.
type TagStats struct {
	ItemCount      int
	ContainerCount int
	TotalQuantity  int
}

// TagDetailData is the view model for the tag detail page.
type TagDetailData struct {
	Tag        store.Tag
	Path       []store.Tag
	Items      []store.Item
	Containers []store.Container
	Stats      TagStats
	Children   []store.Tag
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

func NewServer(s *store.Store, pm *print.PrinterManager, tr *webutil.Translations,
	inventory *service.InventoryService, bulk *service.BulkService,
	tagsSvc *service.TagService, search *service.SearchService,
	printersSvc *service.PrinterService) *Server {

	resolveTagsFn := func(ids []string) []store.Tag {
		tags := make([]store.Tag, 0, len(ids))
		for _, id := range ids {
			if t := tagsSvc.GetTag(id); t != nil {
				tags = append(tags, *t)
			}
		}
		return tags
	}
	templates := loadTemplates(resolveTagsFn)

	staticFS, err := fs.Sub(embedded.Static, "static")
	if err != nil {
		panic(err)
	}

	return &Server{
		store:          s,
		printerManager: pm,
		templates:      templates,
		staticFS:       staticFS,
		translations:   tr,
		inventory:      inventory,
		bulk:           bulk,
		tags:           tagsSvc,
		search:         search,
		printers:       printersSvc,
	}
}

// loadTemplates discovers and parses all HTML templates from the embedded FS.
func loadTemplates(resolveTagsFn func([]string) []store.Tag) map[string]*template.Template {
	layoutTmpl := loadLayout(resolveTagsFn)
	mergeHTMLDir(layoutTmpl, "templates/partials")
	mergeHTMLDir(layoutTmpl, "templates/components")
	return discoverPages(layoutTmpl)
}

func loadLayout(resolveTagsFn func([]string) []store.Tag) *template.Template {
	content, err := embedded.Templates.ReadFile("templates/layouts/base.html")
	if err != nil {
		panic(err)
	}
	return template.Must(template.New("layout").Funcs(template.FuncMap{
		"dict":        dict,
		"resolveTags": resolveTagsFn,
		"icon": func(name string) template.HTML {
			data, err := palette.SVG(name)
			if err != nil {
				return ""
			}
			return template.HTML(data) //nolint:gosec
		},
		"paletteHex": func(name string) string {
			c, ok := palette.ColorByName(name)
			if !ok {
				return ""
			}
			return c.Hex
		},
		"allColors":      palette.AllColors,
		"iconCategories": palette.IconCategories,
	}).Parse(string(content)))
}

// mergeHTMLDir walks a directory and parses all .html files into tmpl.
func mergeHTMLDir(tmpl *template.Template, dir string) {
	err := fs.WalkDir(embedded.Templates, dir, func(fpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(fpath) != ".html" {
			return walkErr
		}
		content, err := embedded.Templates.ReadFile(fpath)
		if err != nil {
			return err
		}
		template.Must(tmpl.Parse(string(content)))
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// discoverPages walks templates/pages/ and registers each .html as a named template.
func discoverPages(layoutTmpl *template.Template) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	err := fs.WalkDir(embedded.Templates, "templates/pages", func(fpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(fpath) != ".html" {
			return walkErr
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
	return templates
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
	mux.HandleFunc("GET /ui/containers/{id}/edit", s.HandleContainerEdit)
	mux.HandleFunc("GET /ui/items/{id}/edit", s.HandleItemEdit)

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
	mux.HandleFunc("GET /ui/tags/{id}", s.HandleTagView)
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

	// Settings
	mux.HandleFunc("GET /ui/settings", s.HandleSettings)
	mux.HandleFunc("POST /ui/actions/set-lang", s.HandleSetLang)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := "pl"
	if v := r.Context().Value(webutil.LangKey); v != nil {
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
	if v := r.Context().Value(webutil.LangKey); v != nil {
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

// HandleSettings renders the settings page.
func (s *Server) HandleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings", nil)
}

// HandleSetLang sets the lang cookie and redirects back to the referer.
func (s *Server) HandleSetLang(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("lang") //nolint:gosec // G120: internal tool, no untrusted input
	if lang == "" {
		lang = "pl"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/ui"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

func (s *Server) containerViewModel(containerID string) (ContainerListData, bool) {
	printersList := s.printers.AllPrinters()
	templatesList := s.store.AllTemplates()

	schemas := label.SchemaNames()

	if containerID == "" {
		return ContainerListData{
			Children:  s.inventory.ContainerChildren(""),
			Printers:  printersList,
			Templates: templatesList,
			Schemas:   schemas,
		}, true
	}

	container := s.inventory.GetContainer(containerID)
	if container == nil {
		return ContainerListData{}, false
	}

	return ContainerListData{
		Children:  s.inventory.ContainerChildren(containerID),
		Items:     s.inventory.ContainerItems(containerID),
		Container: container,
		Path:      s.inventory.ContainerPath(containerID),
		Printers:  printersList,
		Templates: templatesList,
		Schemas:   schemas,
	}, true
}

func (s *Server) itemDetailViewModel(itemID string) (ItemDetailData, bool) {
	item := s.inventory.GetItem(itemID)
	if item == nil {
		return ItemDetailData{}, false
	}

	return ItemDetailData{
		Item:      item,
		Path:      s.inventory.ContainerPath(item.ContainerID),
		Printers:  s.printers.AllPrinters(),
		Templates: s.store.AllTemplates(),
		Schemas:   label.SchemaNames(),
	}, true
}

func (s *Server) printersViewModel() PrintersData {
	printersList := s.printers.AllPrinters()
	var encoders []EncoderData
	for name, enc := range s.printerManager.AvailableEncoders() {
		encoders = append(encoders, EncoderData{
			Name:   name,
			Models: enc.Models(),
		})
	}
	return PrintersData{
		Printers: printersList,
		Encoders: encoders,
	}
}
