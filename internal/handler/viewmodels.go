package handler

import (
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// PageData is the top-level template context for all page renders.
type PageData struct {
	Lang       string
	translator *webutil.Translations
	Data       any
}

// T returns the translation for key in the active language.
func (p PageData) T(key string) string {
	return p.translator.Get(p.Lang, key)
}

// ContainerListData is the view model for the container list page.
type ContainerListData struct {
	Children  []store.Container
	Items     []store.Item
	Container *store.Container
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
	Schemas   []string
	NoteCount int
}

// ItemDetailData is the view model for the item detail page.
type ItemDetailData struct {
	Item      *store.Item
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
	Schemas   []string
	NoteCount int
}

// PrintersData is the view model for the printers page.
type PrintersData struct {
	Printers []store.PrinterConfig
	Encoders []EncoderData
}

// EncoderData represents an encoder and its supported models.
type EncoderData struct {
	Name   string
	Models []encoder.ModelInfo
}

// TemplateListData is the view model for the template list page.
type TemplateListData struct {
	Templates []store.Template
	Tags      []string
	ActiveTag string
}

// TagTreeData is the view model for the tag tree page.
type TagTreeData struct {
	Tags         []store.Tag
	Parent       *store.Tag
	Path         []store.Tag
	ChildCounts  map[string]int // tag ID → number of direct children
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

// SearchResultsData is the view model for search results.
type SearchResultsData struct {
	Query      string
	Containers []store.Container
	Items      []store.Item
	Tags       []store.Tag
	Notes      []store.Note
}

// TagChipsData is the view model for tag chips partial.
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

// DesignerData is the view model for the template designer page.
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

// QuickPrintData is the view model for the quick print page.
type QuickPrintData struct {
	Printers []store.PrinterConfig
	Schemas  []string
}

// DebugToolsData is the view model for the debug tools page.
type DebugToolsData struct {
	Printers []store.PrinterConfig
	Schemas  []string
	Fonts    []string
}

// SettingsData is the view model for the settings page.
type SettingsData struct{}

// NotesTabData is the view model for the notes tab partial.
type NotesTabData struct {
	Notes       []store.Note
	ContainerID string
	ItemID      string
	// ParentType is "container" or "item" — used in the hidden input name.
	ParentType string
	// ParentID is the ID of the parent container or item.
	ParentID string
}
