# Frontend & Backend Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modularize monolithic JS/CSS, establish i18n foundations, unify error handling, and introduce a testable service layer — all without changing external behavior.

**Architecture:** Six sequential steps: (1) template restructure with PageData wrapper, (2) i18n scaffold with cookie+Accept-Language middleware, (3) JS split into domain modules with `window.qlx` namespace, (4) CSS split with design tokens, (5) unified error mapping, (6) service layer with store interfaces. Steps 3+4 can run in parallel.

**Tech Stack:** Go 1.25, html/template, vanilla JS with @ts-check/JSDoc, vanilla CSS with custom properties, go:embed

**Spec:** `docs/superpowers/specs/2026-03-21-frontend-backend-cleanup-design.md`

---

## Task 1: Move templates into domain folders

**Files:**
- Move: `internal/embedded/templates/layout.html` → `internal/embedded/templates/layouts/base.html`
- Move: `internal/embedded/templates/containers.html` → `internal/embedded/templates/pages/inventory/containers.html`
- Move: `internal/embedded/templates/container_form.html` → `internal/embedded/templates/pages/inventory/container_form.html`
- Move: `internal/embedded/templates/item.html` → `internal/embedded/templates/pages/inventory/item.html`
- Move: `internal/embedded/templates/item_form.html` → `internal/embedded/templates/pages/inventory/item_form.html`
- Move: `internal/embedded/templates/printers.html` → `internal/embedded/templates/pages/printers/printers.html`
- Move: `internal/embedded/templates/templates.html` → `internal/embedded/templates/pages/labels/templates.html`
- Move: `internal/embedded/templates/template_designer.html` → `internal/embedded/templates/pages/labels/template_designer.html`
- Move: `internal/embedded/templates/tags.html` → `internal/embedded/templates/pages/tags/tags.html`
- Move: `internal/embedded/templates/search.html` → `internal/embedded/templates/pages/search/search.html`
- Move: `internal/embedded/templates/partials/breadcrumb.html` → `internal/embedded/templates/partials/inventory/breadcrumb.html`
- Move: `internal/embedded/templates/partials/container_list_item.html` → `internal/embedded/templates/partials/inventory/container_list_item.html`
- Move: `internal/embedded/templates/partials/item_list_item.html` → `internal/embedded/templates/partials/inventory/item_list_item.html`
- Move: `internal/embedded/templates/partials/tree_children.html` → `internal/embedded/templates/partials/inventory/tree_children.html`
- Move: `internal/embedded/templates/partials/tag_chips.html` → `internal/embedded/templates/partials/tags/tag_chips.html`
- Move: `internal/embedded/templates/partials/tag_list_item.html` → `internal/embedded/templates/partials/tags/tag_list_item.html`
- Move: `internal/embedded/templates/partials/tag_tree_children.html` → `internal/embedded/templates/partials/tags/tag_tree_children.html`
- Keep: `internal/embedded/templates/components/form_fields.html` (stays as-is)
- Modify: `internal/embedded/embedded.go`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p internal/embedded/templates/{layouts,pages/{inventory,printers,labels,tags,search},partials/{inventory,tags},components}
```

- [ ] **Step 2: Move all template files**

```bash
# Layout
mv internal/embedded/templates/layout.html internal/embedded/templates/layouts/base.html

# Pages
mv internal/embedded/templates/containers.html internal/embedded/templates/pages/inventory/
mv internal/embedded/templates/container_form.html internal/embedded/templates/pages/inventory/
mv internal/embedded/templates/item.html internal/embedded/templates/pages/inventory/
mv internal/embedded/templates/item_form.html internal/embedded/templates/pages/inventory/
mv internal/embedded/templates/printers.html internal/embedded/templates/pages/printers/
mv internal/embedded/templates/templates.html internal/embedded/templates/pages/labels/
mv internal/embedded/templates/template_designer.html internal/embedded/templates/pages/labels/
mv internal/embedded/templates/tags.html internal/embedded/templates/pages/tags/
mv internal/embedded/templates/search.html internal/embedded/templates/pages/search/

# Partials
mv internal/embedded/templates/partials/breadcrumb.html internal/embedded/templates/partials/inventory/
mv internal/embedded/templates/partials/container_list_item.html internal/embedded/templates/partials/inventory/
mv internal/embedded/templates/partials/item_list_item.html internal/embedded/templates/partials/inventory/
mv internal/embedded/templates/partials/tree_children.html internal/embedded/templates/partials/inventory/
mv internal/embedded/templates/partials/tag_chips.html internal/embedded/templates/partials/tags/
mv internal/embedded/templates/partials/tag_list_item.html internal/embedded/templates/partials/tags/
mv internal/embedded/templates/partials/tag_tree_children.html internal/embedded/templates/partials/tags/
```

- [ ] **Step 3: Update `embedded.go` to use whole-dir embed**

Change `internal/embedded/embedded.go` from:
```go
//go:embed templates/*.html templates/partials/*.html templates/components/*.html
var Templates embed.FS
```
to:
```go
//go:embed templates
var Templates embed.FS

//go:embed static
var Static embed.FS
```

This embeds the entire `templates/` and `static/` trees including all subdirectories. The previous `//go:embed static/*` glob does NOT recurse into subdirectories — since we're adding `static/js/shared/`, `static/css/shared/`, etc., we must use `//go:embed static` (whole dir) to capture nested files.

- [ ] **Step 4: Update `NewServer` template loading to walk subdirectories**

In `internal/ui/server.go`, replace the hardcoded `templateFiles` map and `sharedFiles` list with `fs.WalkDir`-based discovery:

```go
func NewServer(s *store.Store, pm *print.PrinterManager) *Server {
	// Read layout
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

	// Load all partials and components into layout template
	for _, dir := range []string{"templates/partials", "templates/components"} {
		_ = fs.WalkDir(embedded.Templates, dir, func(fpath string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(fpath, ".html") {
				return err
			}
			content, err := embedded.Templates.ReadFile(fpath)
			if err != nil {
				panic(err)
			}
			layoutTmpl = template.Must(layoutTmpl.Parse(string(content)))
			return nil
		})
	}

	// Load page templates
	templates := make(map[string]*template.Template)
	_ = fs.WalkDir(embedded.Templates, "templates/pages", func(fpath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(fpath, ".html") {
			return err
		}
		content, err := embedded.Templates.ReadFile(fpath)
		if err != nil {
			panic(err)
		}
		// Derive name from filename without extension: "containers", "item", etc.
		base := strings.TrimSuffix(path.Base(fpath), ".html")
		// Map old names: "container_form" → "container-form"
		name := strings.ReplaceAll(base, "_", "-")

		tmpl, err := layoutTmpl.Clone()
		if err != nil {
			panic(err)
		}
		tmpl = template.Must(tmpl.Parse(string(content)))
		wrapper := `{{ define "content" }}{{ template "` + name + `" . }}{{ end }}`
		tmpl = template.Must(tmpl.Parse(wrapper))
		templates[name] = tmpl
		return nil
	})

	// ... rest unchanged
}
```

Add `"io/fs"`, `"path"`, `"strings"` to imports if not already present. Use `path.Base()` (not `filepath.Base()`) because `embed.FS` always uses forward slashes per `io/fs` spec.

- [ ] **Step 5: Verify the app compiles and runs**

```bash
make build-mac && make run
```

Navigate to `http://localhost:8080/ui` — all pages should render identically.

- [ ] **Step 6: Run E2E tests**

```bash
make test-e2e
```

All existing tests must pass — this is a pure file move with no behavior change.

- [ ] **Step 7: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor(ui): move templates into domain folder structure

Move flat template files into pages/, partials/, components/ subdirectories
organized by domain (inventory, printers, labels, tags, search).
Template loading now uses fs.WalkDir for automatic discovery."
```

---

## Task 2: Add PageData wrapper and update render/renderPartial

**Files:**
- Modify: `internal/ui/server.go` — add `PageData` struct, update `render()` and `renderPartial()`
- Modify: `internal/ui/handlers.go` — update `renderPartial` call sites to pass `r`
- Modify: `internal/ui/handlers_tags.go` — update `renderPartial` call sites
- Modify: `internal/ui/handlers_partials.go` — update `renderPartial` call sites

- [ ] **Step 1: Add PageData struct and stub Translations**

Add to `internal/ui/server.go` after the existing type declarations:

```go
// langKey is the context key for language.
type contextKey string
const langKey contextKey = "lang"

// Translations holds i18n strings per language.
type Translations struct {
	langs map[string]map[string]string
}

func NewTranslations() *Translations {
	return &Translations{langs: map[string]map[string]string{
		"pl": {},
		"en": {},
	}}
}

func (t *Translations) Get(lang, key string) string {
	if val, ok := t.langs[lang][key]; ok {
		return val
	}
	if val, ok := t.langs["en"][key]; ok {
		return val
	}
	return key
}

// PageData wraps per-page data with per-request context.
type PageData struct {
	Lang       string
	translator *Translations
	Data       any
}

func (p PageData) T(key string) string {
	return p.translator.Get(p.Lang, key)
}
```

Add `translations *Translations` field to `Server` struct. Initialize in `NewServer`:

```go
return &Server{store: s, printerManager: pm, templates: templates, staticFS: staticFS, translations: NewTranslations()}
```

- [ ] **Step 2: Update render() to wrap data in PageData**

Replace the `render` method in `internal/ui/server.go`:

```go
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := "pl" // default until middleware is wired
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
```

- [ ] **Step 3: Update renderPartial() to accept `*http.Request` and wrap data**

Replace `renderPartial` in `internal/ui/server.go`:

```go
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
```

- [ ] **Step 4: Update all renderPartial call sites to pass `r`**

In `internal/ui/handlers.go`, find all `s.renderPartial(w,` calls and add `r` parameter:
- `s.renderPartial(w, "containers", "container-list-item", container)` → `s.renderPartial(w, r, "containers", "container-list-item", container)`
- `s.renderPartial(w, "containers", "item-list-item", item)` → `s.renderPartial(w, r, "containers", "item-list-item", item)`

In `internal/ui/handlers_partials.go`, all 4 calls:
- `s.renderPartial(w, "containers", "tree-children", children)` → `s.renderPartial(w, r, "containers", "tree-children", children)`
- `s.renderPartial(w, "containers", "tree-children", results)` → `s.renderPartial(w, r, "containers", "tree-children", results)`
- `s.renderPartial(w, "tags", "tag-tree-children", children)` → `s.renderPartial(w, r, "tags", "tag-tree-children", children)`
- `s.renderPartial(w, "tags", "tag-tree-children", results)` → `s.renderPartial(w, r, "tags", "tag-tree-children", results)`

In `internal/ui/handlers_tags.go`, find all `s.renderPartial(w,` calls and add `r`.

- [ ] **Step 5: Update ALL templates to use `.Data.` prefix**

Every template that accesses page-specific data must be prefixed with `.Data.`. Examples:

`containers.html`: `{{range .Children}}` → `{{range .Data.Children}}`, `{{.Container.Name}}` → `{{.Data.Container.Name}}`, etc.

`item.html`: `{{.Item.Name}}` → `{{.Data.Item.Name}}`, etc.

`printers.html`: `{{range .Printers}}` → `{{range .Data.Printers}}`, etc.

Apply to ALL page templates and ALL partials. In partials the data passed is already the specific type (e.g., `[]Container`), so partials access `.Data` directly:
- `{{range .}}` → `{{range .Data}}` (for partials that iterate over a slice)

**Important:** The `breadcrumb.html` partial uses `dict` to pass data. In the calling template, update: `{{ template "breadcrumb" dict "Path" .Path }}` → `{{ template "breadcrumb" dict "Path" .Data.Path }}`. The breadcrumb partial itself receives a map, so its internal references (`{{range .Path}}`) stay the same.

- [ ] **Step 6: Verify compilation and all pages render**

```bash
make build-mac && make run
```

Check all pages: `/ui`, `/ui/containers/{id}`, `/ui/items/{id}`, `/ui/printers`, `/ui/tags`, `/ui/templates`, `/ui/search?q=test`.

- [ ] **Step 7: Run E2E tests**

```bash
make test-e2e
```

- [ ] **Step 8: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor(ui): add PageData wrapper for i18n-ready templates

Wrap all template data in PageData{Lang, T, Data}. Templates now access
page data via .Data prefix and can use .T for future translations.
renderPartial() now accepts *http.Request for language context."
```

---

## Task 3: i18n middleware and translation loader

**Files:**
- Create: `internal/shared/webutil/i18n.go`
- Create: `internal/embedded/static/i18n/en/nav.json`
- Create: `internal/embedded/static/i18n/en/actions.json`
- Create: `internal/embedded/static/i18n/pl/nav.json`
- Create: `internal/embedded/static/i18n/pl/actions.json`
- Modify: `internal/ui/server.go` — move Translations to webutil, load from embedded
- Modify: `internal/app/server.go` — wire LangMiddleware
- Modify: `internal/embedded/templates/layouts/base.html` — `<html lang="{{.Lang}}">`

- [ ] **Step 1: Create i18n middleware in `internal/shared/webutil/i18n.go`**

```go
package webutil

import (
	"context"
	"net/http"
	"strings"
)

type LangContextKey string

const LangKey LangContextKey = "lang"

// LangMiddleware detects language from cookie, Accept-Language, or default.
func LangMiddleware(defaultLang string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lang := defaultLang
			if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
				lang = c.Value
			} else {
				if parsed := parseAcceptLanguage(r); parsed != "" {
					lang = parsed
				}
			}
			ctx := context.WithValue(r.Context(), LangKey, lang)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func parseAcceptLanguage(r *http.Request) string {
	header := r.Header.Get("Accept-Language")
	if header == "" {
		return ""
	}
	// Simple parser: take first language tag
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if len(tag) >= 2 {
			return tag[:2] // "en-US" → "en"
		}
	}
	return ""
}
```

- [ ] **Step 2: Create initial translation files**

Create `internal/embedded/static/i18n/en/nav.json`:
```json
{
  "nav.home": "Inventory",
  "nav.printers": "Printers",
  "nav.templates": "Templates",
  "nav.tags": "Tags",
  "nav.search_placeholder": "Search...",
  "nav.title": "QLX - Labels"
}
```

Create `internal/embedded/static/i18n/en/actions.json`:
```json
{
  "action.delete": "Delete",
  "action.cancel": "Cancel",
  "action.move": "Move",
  "action.save": "Save"
}
```

Create `internal/embedded/static/i18n/pl/nav.json`:
```json
{
  "nav.home": "Magazyn",
  "nav.printers": "Drukarki",
  "nav.templates": "Szablony",
  "nav.tags": "Tagi",
  "nav.search_placeholder": "Szukaj...",
  "nav.title": "QLX - Etykiety"
}
```

Create `internal/embedded/static/i18n/pl/actions.json`:
```json
{
  "action.delete": "Usuń",
  "action.cancel": "Anuluj",
  "action.move": "Przenieś",
  "action.save": "Zapisz"
}
```

Create `internal/embedded/static/i18n/en/errors.json`:
```json
{
  "error.connection": "Connection error",
  "error.status": "Error",
  "error.not_found": "Not found"
}
```

Create `internal/embedded/static/i18n/pl/errors.json`:
```json
{
  "error.connection": "Błąd połączenia",
  "error.status": "Błąd",
  "error.not_found": "Nie znaleziono"
}
```

- [ ] **Step 3: Move Translations to webutil and add file loader**

Move the `Translations` struct from `ui/server.go` to `internal/shared/webutil/i18n.go` and add a loader:

```go
// Translations holds i18n strings per language with fallback to "en".
type Translations struct {
	langs map[string]map[string]string
}

func NewTranslations() *Translations {
	return &Translations{langs: make(map[string]map[string]string)}
}

func (t *Translations) Get(lang, key string) string {
	if val, ok := t.langs[lang][key]; ok {
		return val
	}
	if val, ok := t.langs["en"][key]; ok {
		return val
	}
	return key
}

// Merged returns all translations for a language merged with en fallback.
func (t *Translations) Merged(lang string) map[string]string {
	merged := make(map[string]string)
	for k, v := range t.langs["en"] {
		merged[k] = v
	}
	for k, v := range t.langs[lang] {
		merged[k] = v
	}
	return merged
}

// LoadFromFS loads all .json files from i18n/{lang}/ directories.
func (t *Translations) LoadFromFS(fsys fs.FS, root string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		// path is like "static/i18n/en/nav.json"
		parts := strings.Split(path, "/")
		// Find the lang part: it's the directory right after "i18n"
		var lang string
		for i, p := range parts {
			if p == "i18n" && i+1 < len(parts) {
				lang = parts[i+1]
				break
			}
		}
		if lang == "" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		var entries map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if t.langs[lang] == nil {
			t.langs[lang] = make(map[string]string)
		}
		for k, v := range entries {
			t.langs[lang][k] = v
		}
		return nil
	})
}
```

Add imports: `"encoding/json"`, `"fmt"`, `"io/fs"`.

- [ ] **Step 4: Update ui/server.go to load translations from embedded FS**

In `NewServer`, replace `NewTranslations()` with:

```go
translations := webutil.NewTranslations()
if err := translations.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
	panic(err)
}
```

Update `Server` struct field: `translations *webutil.Translations`.

Update `PageData`:
```go
type PageData struct {
	Lang       string
	translator *webutil.Translations
	Data       any
}

func (p PageData) T(key string) string {
	return p.translator.Get(p.Lang, key)
}
```

Update `langKey` usage to use `webutil.LangKey`:
```go
if v := r.Context().Value(webutil.LangKey); v != nil {
    lang = v.(string)
}
```

Remove the local `Translations`, `NewTranslations`, `contextKey`, `langKey` definitions.

- [ ] **Step 5: Wire LangMiddleware in app/server.go**

In `internal/app/server.go`, wrap the handler:

```go
return &Server{handler: webutil.LangMiddleware("pl")(webutil.LoggingMiddleware(mux))}
```

- [ ] **Step 6: Add /api/i18n/{lang} endpoint**

In `internal/api/server.go`, add to `RegisterRoutes`:

```go
mux.HandleFunc("GET /api/i18n/{lang}", s.HandleI18n)
```

Add the handler (api server needs access to translations — pass via constructor or load independently):

```go
func (s *Server) HandleI18n(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	merged := s.translations.Merged(lang)
	webutil.JSON(w, http.StatusOK, merged)
}
```

Update API `Server` struct and constructor:

```go
type Server struct {
	store          *store.Store
	printerManager *print.PrinterManager
	translations   *webutil.Translations
}

func NewServer(s *store.Store, pm *print.PrinterManager, tr *webutil.Translations) *Server {
	return &Server{store: s, printerManager: pm, translations: tr}
}
```

Update `internal/app/server.go` to pass translations:

```go
func NewServer(s *store.Store, pm *qlprint.PrinterManager) *Server {
	translations := webutil.NewTranslations()
	if err := translations.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		panic(err)
	}

	uiServer := ui.NewServer(s, pm, translations)
	apiServer := api.NewServer(s, pm, translations)
	// ... rest unchanged
}
```

Update `ui.NewServer` signature similarly to accept `*webutil.Translations`.

- [ ] **Step 6.5: Add language switch endpoint**

In `internal/ui/server.go`, add to `RegisterRoutes`:

```go
mux.HandleFunc("POST /ui/actions/set-lang", s.HandleSetLang)
```

Add handler:

```go
func (s *Server) HandleSetLang(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("lang")
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
```

- [ ] **Step 6.6: Write i18n unit tests**

Create `internal/shared/webutil/i18n_test.go`:

```go
package webutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestTranslations_Get_Fallback(t *testing.T) {
	tr := NewTranslations()
	tr.langs["en"] = map[string]string{"hello": "Hello"}
	tr.langs["pl"] = map[string]string{"hello": "Cześć"}

	if got := tr.Get("pl", "hello"); got != "Cześć" {
		t.Errorf("Get(pl, hello) = %q, want Cześć", got)
	}
	if got := tr.Get("pl", "missing"); got != "missing" {
		t.Errorf("Get(pl, missing) = %q, want missing", got)
	}
	if got := tr.Get("de", "hello"); got != "Hello" {
		t.Errorf("Get(de, hello) = %q, want Hello (en fallback)", got)
	}
}

func TestTranslations_LoadFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"i18n/en/nav.json": {Data: []byte(`{"nav.home":"Home"}`)},
		"i18n/pl/nav.json": {Data: []byte(`{"nav.home":"Magazyn"}`)},
	}
	tr := NewTranslations()
	if err := tr.LoadFromFS(fsys, "i18n"); err != nil {
		t.Fatal(err)
	}
	if got := tr.Get("en", "nav.home"); got != "Home" {
		t.Errorf("Get(en) = %q, want Home", got)
	}
	if got := tr.Get("pl", "nav.home"); got != "Magazyn" {
		t.Errorf("Get(pl) = %q, want Magazyn", got)
	}
}

func TestLangMiddleware_Cookie(t *testing.T) {
	handler := LangMiddleware("pl")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Context().Value(LangKey).(string)
		w.Write([]byte(lang))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "en"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Body.String() != "en" {
		t.Errorf("got %q, want en", rec.Body.String())
	}
}

func TestLangMiddleware_Default(t *testing.T) {
	handler := LangMiddleware("pl")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Context().Value(LangKey).(string)
		w.Write([]byte(lang))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Body.String() != "pl" {
		t.Errorf("got %q, want pl", rec.Body.String())
	}
}
```

Run: `go test ./internal/shared/webutil/ -run TestTranslations -v && go test ./internal/shared/webutil/ -run TestLangMiddleware -v`

- [ ] **Step 7: Update base.html with dynamic lang and translated nav**

In `internal/embedded/templates/layouts/base.html`:

```html
<html lang="{{.Lang}}">
```

Replace hardcoded nav strings:
```html
<title>{{.T "nav.title"}}</title>
```
```html
<a href="/ui/printers" hx-get="/ui/printers" hx-target="#content">{{.T "nav.printers"}}</a>
<a href="/ui/templates" hx-get="/ui/templates" hx-target="#content">{{.T "nav.templates"}}</a>
<a href="/ui/tags" hx-get="/ui/tags" hx-target="#content">{{.T "nav.tags"}}</a>
<input type="search" id="global-search" placeholder="{{.T "nav.search_placeholder"}}"
```

- [ ] **Step 8: Verify compilation, run, and test**

```bash
make build-mac && make run
```

Check that nav renders in Polish (default). Test with browser language set to English.

```bash
make test-e2e
```

- [ ] **Step 9: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "feat(i18n): add translation middleware and initial PL/EN translations

Add LangMiddleware (cookie > Accept-Language > default 'pl'),
Translations loader from embedded JSON files, /api/i18n/{lang} endpoint.
Nav strings in base.html now use {{.T}} for translation."
```

---

## Task 4: Split ui-lite.js into domain modules

**Files:**
- Create: `internal/embedded/static/js/shared/namespace.js`
- Create: `internal/embedded/static/js/shared/i18n.js`
- Create: `internal/embedded/static/js/shared/toast.js`
- Create: `internal/embedded/static/js/shared/sse.js`
- Create: `internal/embedded/static/js/shared/htmx-hooks.js`
- Create: `internal/embedded/static/js/inventory/selection.js`
- Create: `internal/embedded/static/js/inventory/dragdrop.js`
- Create: `internal/embedded/static/js/inventory/move-picker.js`
- Create: `internal/embedded/static/js/inventory/delete-confirm.js`
- Create: `internal/embedded/static/js/tags/tag-picker.js`
- Create: `internal/embedded/static/js/labels/template-filter.js`
- Delete: `internal/embedded/static/ui-lite.js`
- Modify: `internal/embedded/templates/layouts/base.html` — replace single script with multiple

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p internal/embedded/static/js/{shared,inventory,tags,labels}
```

- [ ] **Step 2: Create `namespace.js`**

```js
// @ts-check
window.qlx = {};
```

- [ ] **Step 3: Create `i18n.js`**

Extract from spec. Fetches translations from `/api/i18n/{lang}`:

```js
// @ts-check
(function () {
  var qlx = window.qlx = window.qlx || {};
  var strings = {};

  var lang = document.documentElement.lang || "pl";
  fetch("/api/i18n/" + lang)
    .then(function (r) { return r.json(); })
    .then(function (data) { strings = data; });

  /** @param {string} key @returns {string} */
  qlx.t = function (key) {
    return strings[key] || key;
  };
})();
```

- [ ] **Step 4: Create `toast.js`**

Extract lines 14-30 from `ui-lite.js`:

```js
// @ts-check
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * @param {string} message
   * @param {boolean} [isError]
   */
  qlx.showToast = function (message, isError) {
    var container = document.getElementById("toast-container");
    if (!container) {
      container = document.createElement("div");
      container.id = "toast-container";
      document.body.appendChild(container);
    }
    var toast = document.createElement("div");
    toast.className = "toast" + (isError ? " toast-error" : " toast-success");
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(function () {
      toast.classList.add("toast-fade");
      setTimeout(function () { toast.remove(); }, 300);
    }, 3000);
  };
})();
```

- [ ] **Step 5: Create `sse.js`**

Extract lines 772-881 from `ui-lite.js` (SSE, updatePrinterCard, updateNavbarPrinter, fetchInitialStatuses). Keep all functions. The module self-starts SSE on load.

- [ ] **Step 6: Create `htmx-hooks.js`**

Extract lines 2-11 (htmx:afterSwap autofocus) and lines 748-755 (template filter on swap) and lines 879-881 (fetchInitialStatuses on swap). This is the central htmx event coordinator:

```js
// @ts-check
(function () {
  document.body.addEventListener("htmx:afterSettle", function (event) {
    if (!event.detail || !event.detail.target) return;
    var target = event.detail.target;
    if (target.id !== "content") return;
    var autofocus = target.querySelector("[autofocus]");
    if (autofocus) autofocus.focus();
  });
})();
```

- [ ] **Step 7: Create `selection.js`**

Extract lines 32-145 from `ui-lite.js` (selection Map, initBulkSelect, onBulkCheckChange, onSelectToggle, clearSelection, action bar). Expose `qlx.clearSelection` and `qlx.initBulkSelect`. Self-register on `htmx:afterSettle`.

- [ ] **Step 8: Create `dragdrop.js`**

Extract lines 557-713 from `ui-lite.js` (initDragDrop, onDragStart, onDragEnd, onDragOver, onDragLeave, onDrop with single+multi drag). Uses `qlx.showToast`, `qlx.clearSelection`, `qlx.t`. Self-register on `htmx:afterSettle`.

- [ ] **Step 9: Create `move-picker.js`**

Extract lines 148-282 from `ui-lite.js` (getOrCreateMovePickerDialog, openMovePicker, executeBulkMove, handleTreeExpand, handleTreeLabelSelect). Uses `qlx.showToast`, `qlx.clearSelection`, `qlx.t`.

- [ ] **Step 10: Create `delete-confirm.js`**

Extract lines 416-494 from `ui-lite.js` (getOrCreateDeleteDialog, openDeleteConfirm, executeBulkDelete). Uses `qlx.showToast`, `qlx.clearSelection`, `qlx.t`.

- [ ] **Step 11: Create `tag-picker.js`**

Extract lines 284-414 from `ui-lite.js` (getOrCreateTagPickerDialog, openTagPicker, executeBulkTag). Uses `qlx.showToast`, `qlx.clearSelection`, `qlx.t`. Shares tree helpers with move-picker — extract `handleTreeExpand` and `handleTreeLabelSelect` to a shared helper or duplicate (they're short).

- [ ] **Step 12: Create `template-filter.js`**

Extract lines 716-755 from `ui-lite.js` (filterTemplates function). Expose as `qlx.filterTemplates`.

- [ ] **Step 13: Update `base.html` script tags**

Replace:
```html
<script src="/static/ui-lite.js" defer></script>
```
with:
```html
<script src="/static/js/shared/namespace.js" defer></script>
<script src="/static/js/shared/i18n.js" defer></script>
<script src="/static/js/shared/toast.js" defer></script>
<script src="/static/js/shared/sse.js" defer></script>
<script src="/static/js/shared/htmx-hooks.js" defer></script>
<script src="/static/js/inventory/selection.js" defer></script>
<script src="/static/js/inventory/dragdrop.js" defer></script>
<script src="/static/js/inventory/move-picker.js" defer></script>
<script src="/static/js/inventory/delete-confirm.js" defer></script>
<script src="/static/js/tags/tag-picker.js" defer></script>
<script src="/static/js/labels/template-filter.js" defer></script>
```

- [ ] **Step 14: Delete `ui-lite.js`**

```bash
rm internal/embedded/static/ui-lite.js
```

- [ ] **Step 15: Replace hardcoded Polish strings in JS with `qlx.t()` calls**

Go through each new JS file and replace hardcoded strings:
- `"Przenieś do..."` → `qlx.t("action.move_to")`
- `"Zaznaczono: "` → `qlx.t("bulk.selected_count") + ": "`
- `"Anuluj"` → `qlx.t("action.cancel")`
- `"Usuń zaznaczone"` → `qlx.t("bulk.delete_selected")`
- `"Błąd połączenia"` → `qlx.t("error.connection")`
- etc.

Add corresponding keys to `en/actions.json` and `pl/actions.json` (and create `en/errors.json`, `pl/errors.json`, `en/inventory.json`, `pl/inventory.json` as needed).

- [ ] **Step 16: Add `tsconfig.json`**

Create `tsconfig.json` in project root:
```json
{
    "compilerOptions": {
        "checkJs": true,
        "noEmit": true,
        "target": "ES5",
        "strict": true
    },
    "include": ["internal/embedded/static/js/**/*.js"]
}
```

- [ ] **Step 17: Verify tsc passes**

```bash
tsc --noEmit
```

Fix any type errors.

- [ ] **Step 18: Add tsc to Makefile lint**

Add to `Makefile` lint target:
```makefile
lint:
	golangci-lint run ./...
	tsc --noEmit
```

- [ ] **Step 19: Verify everything works**

```bash
make build-mac && make run
```

Test: bulk select, drag & drop, move picker, tag picker, delete confirm, SSE printer status, template filter.

```bash
make test-e2e
```

- [ ] **Step 20: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor(js): split ui-lite.js into domain modules with @ts-check

Split 882-line monolith into shared/ (namespace, i18n, toast, sse, htmx-hooks)
and domain modules (inventory/, tags/, labels/). Each module is a self-contained
IIFE using window.qlx namespace. Add JSDoc annotations and tsconfig.json.
Replace hardcoded Polish strings with qlx.t() calls.
Closes #4, closes #7"
```

---

## Task 5: Split style.css into domain modules with design tokens

**Files:**
- Create: `internal/embedded/static/css/shared/tokens.css`
- Create: `internal/embedded/static/css/shared/reset.css`
- Create: `internal/embedded/static/css/shared/base.css`
- Create: `internal/embedded/static/css/shared/forms.css`
- Create: `internal/embedded/static/css/shared/buttons.css`
- Create: `internal/embedded/static/css/shared/toast.css`
- Create: `internal/embedded/static/css/layout/nav.css`
- Create: `internal/embedded/static/css/layout/content.css`
- Create: `internal/embedded/static/css/layout/responsive.css`
- Create: `internal/embedded/static/css/inventory/lists.css`
- Create: `internal/embedded/static/css/inventory/cards.css`
- Create: `internal/embedded/static/css/inventory/dragdrop.css`
- Create: `internal/embedded/static/css/inventory/selection.css`
- Create: `internal/embedded/static/css/inventory/quick-entry.css`
- Create: `internal/embedded/static/css/tags/tags.css`
- Create: `internal/embedded/static/css/tags/tag-chips.css`
- Create: `internal/embedded/static/css/labels/designer.css`
- Create: `internal/embedded/static/css/labels/template-cards.css`
- Create: `internal/embedded/static/css/dialogs/dialog.css`
- Create: `internal/embedded/static/css/dialogs/tree-picker.css`
- Create: `internal/embedded/static/css/search/search.css`
- Delete: `internal/embedded/static/style.css`
- Modify: `internal/embedded/templates/layouts/base.html`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p internal/embedded/static/css/{shared,layout,inventory,tags,labels,dialogs,search}
```

- [ ] **Step 2: Create `tokens.css` with design tokens**

Extract `:root` block from `style.css` lines 1-12 and expand with new tokens:

```css
:root {
    /* Colors */
    --color-bg: #1a1a2e;
    --color-bg-alt: #16213e;
    --color-bg-card: #0f3460;
    --color-text: #eee;
    --color-text-muted: #888;
    --color-accent: #e94560;
    --color-accent-hover: #ff6b6b;
    --color-border: #333;
    --color-success: #4ecca3;
    --color-warning: #ffc93c;

    /* Spacing */
    --space-xs: 0.25rem;
    --space-sm: 0.5rem;
    --space-md: 1rem;
    --space-lg: 1.5rem;
    --space-xl: 2rem;

    /* Typography */
    --font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    --font-size-sm: 0.85rem;
    --font-size-base: 1rem;
    --font-size-lg: 1.1rem;
    --font-size-xl: 1.3rem;
    --font-size-h1: 1.5rem;
    --font-size-h2: 1.2rem;
    --line-height: 1.6;

    /* Borders */
    --radius-sm: 4px;
    --radius-md: 8px;
    --radius-pill: 12px;

    /* Shadows */
    --shadow-card: none;

    /* Z-index */
    --z-toast: 1000;
    --z-action-bar: 100;
}
```

- [ ] **Step 3: Extract remaining CSS into domain files**

Split `style.css` mechanically by comment sections and selectors:
- `reset.css`: lines 14-16 (box-sizing)
- `base.css`: lines 18-25, 60-69 (body, headings, links)
- `forms.css`: lines 177-251 (form, .form-group, inputs)
- `buttons.css`: lines 185-298 (.btn, .btn-*, button.danger, a.button)
- `toast.css`: lines 411-442 (#toast-container, .toast-*)
- `nav.css`: lines 27-52 (nav, .brand, #printer-status)
- `content.css`: lines 54-58, 333-357 (#content, .empty, .section, .section-header)
- `lists.css`: lines 92-131 (.container-list, .item-list)
- `cards.css`: lines 133-155, 444-462 (.item-detail, .printer-card)
- `dragdrop.css`: lines 385-409 ([draggable], .dragging, .drag-over)
- `selection.css`: lines 786-798 (.bulk-select, .action-bar, .selection-mode)
- `quick-entry.css`: lines 836-893 (.quick-entry-*)
- `tags.css`: lines 465-493, 901-906 (.tag, .tag-filter-bar, .tag-list)
- `tag-chips.css`: lines 820-833 (.tag-chip, .tag-add)
- `designer.css`: lines 533-759 (.designer-*)
- `template-cards.css`: lines 505-531 (.template-card, .template-actions, .badge)
- `dialog.css`: lines 800-806 (dialog, ::backdrop)
- `tree-picker.css`: lines 807-818 (.tree-picker, .tree-*)
- `search.css`: lines 895-899 (#global-search)
- `responsive.css`: lines 908-952 (@media queries)

In each file, replace old variable names (`--bg`, `--accent`, etc.) with token names (`--color-bg`, `--color-accent`, etc.).

- [ ] **Step 4: Update `base.html` link tags**

Replace:
```html
<link rel="stylesheet" href="/static/style.css">
```
with all CSS link tags per the spec ordering (shared → layout → domain → responsive last).

- [ ] **Step 5: Delete `style.css`**

```bash
rm internal/embedded/static/style.css
```

- [ ] **Step 6: Verify visual parity**

```bash
make build-mac && make run
```

Check every page visually: containers, items, printers, tags, templates, template designer, search. Confirm dark theme, spacing, dialog styling all match.

- [ ] **Step 7: Run E2E tests**

```bash
make test-e2e
```

- [ ] **Step 8: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor(css): split style.css into domain modules with design tokens

Split 952-line monolith into shared/ (tokens, reset, base, forms, buttons, toast),
layout/ (nav, content, responsive), and domain modules (inventory/, tags/, labels/,
dialogs/, search/). Introduce --color-*, --space-*, --radius-* design tokens.
Closes #8"
```

---

## Task 6: Unified error-to-HTTP mapping

**Files:**
- Create: `internal/shared/webutil/errors.go`
- Test: `internal/shared/webutil/errors_test.go`
- Modify: `internal/api/server.go` — remove `writeStoreError`, use `WriteStoreErrorJSON`
- Modify: `internal/api/handlers_tags.go` — remove `writeTagError`, use `WriteStoreErrorJSON`
- Modify: `internal/ui/handlers.go` — replace raw `http.Error` with `WriteStoreErrorText`
- Modify: `internal/ui/handlers_tags.go` — use `WriteStoreErrorText`
- Modify: `internal/ui/handlers_bulk.go` — use `WriteStoreErrorText`

- [ ] **Step 1: Write tests for error mapping**

Create `internal/shared/webutil/errors_test.go`:

```go
package webutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestStoreHTTPStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{store.ErrContainerNotFound, 404},
		{store.ErrItemNotFound, 404},
		{store.ErrTagNotFound, 404},
		{store.ErrPrinterNotFound, 404},
		{store.ErrContainerHasChildren, 409},
		{store.ErrContainerHasItems, 409},
		{store.ErrTagHasChildren, 409},
		{store.ErrCycleDetected, 400},
		{store.ErrInvalidParent, 400},
		{store.ErrInvalidContainer, 400},
	}
	for _, tt := range tests {
		if got := StoreHTTPStatus(tt.err); got != tt.status {
			t.Errorf("StoreHTTPStatus(%v) = %d, want %d", tt.err, got, tt.status)
		}
	}
}

func TestWriteStoreErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteStoreErrorJSON(w, store.ErrItemNotFound)
	if w.Code != 404 {
		t.Errorf("got status %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}
}

func TestWriteStoreErrorText(t *testing.T) {
	w := httptest.NewRecorder()
	WriteStoreErrorText(w, store.ErrContainerHasChildren)
	if w.Code != 409 {
		t.Errorf("got status %d, want 409", w.Code)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/shared/webutil/ -run TestStore -v
```

Expected: FAIL (functions don't exist yet).

- [ ] **Step 3: Implement `errors.go`**

Create `internal/shared/webutil/errors.go`:

```go
package webutil

import (
	"errors"
	"net/http"

	"github.com/erxyi/qlx/internal/store"
)

var statusMap = map[error]int{
	store.ErrContainerNotFound:    http.StatusNotFound,
	store.ErrItemNotFound:         http.StatusNotFound,
	store.ErrTagNotFound:          http.StatusNotFound,
	store.ErrPrinterNotFound:      http.StatusNotFound,
	store.ErrContainerHasChildren: http.StatusConflict,
	store.ErrContainerHasItems:    http.StatusConflict,
	store.ErrTagHasChildren:       http.StatusConflict,
	store.ErrCycleDetected:        http.StatusBadRequest,
	store.ErrInvalidParent:        http.StatusBadRequest,
	store.ErrInvalidContainer:     http.StatusBadRequest,
}

// StoreHTTPStatus maps a store error to an HTTP status code.
func StoreHTTPStatus(err error) int {
	for sentinel, code := range statusMap {
		if errors.Is(err, sentinel) {
			return code
		}
	}
	return http.StatusBadRequest
}

// WriteStoreErrorJSON writes a JSON error response with the mapped status code.
func WriteStoreErrorJSON(w http.ResponseWriter, err error) {
	JSON(w, StoreHTTPStatus(err), map[string]string{"error": err.Error()})
}

// WriteStoreErrorText writes a plain text error response with the mapped status code.
func WriteStoreErrorText(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), StoreHTTPStatus(err))
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/shared/webutil/ -run TestStore -v
```

- [ ] **Step 5: Replace `writeStoreError` in `api/server.go`**

Delete the `writeStoreError` function. Replace all calls:
- `writeStoreError(w, err)` → `webutil.WriteStoreErrorJSON(w, err)`

- [ ] **Step 6: Replace `writeTagError` in `api/handlers_tags.go`**

Delete the `writeTagError` function. Replace all calls:
- `writeTagError(w, err)` → `webutil.WriteStoreErrorJSON(w, err)`

- [ ] **Step 7: Replace raw `http.Error` in UI handlers**

In `internal/ui/handlers.go`, `handlers_tags.go`, `handlers_bulk.go`:
- `http.Error(w, err.Error(), http.StatusNotFound)` → `webutil.WriteStoreErrorText(w, err)` (where err is a store error)
- `http.Error(w, err.Error(), http.StatusBadRequest)` → `webutil.WriteStoreErrorText(w, err)` (where err is a store error)

Leave non-store errors (e.g., JSON parse errors, template not found) as-is.

- [ ] **Step 8: Run all tests**

```bash
make test
make test-e2e
```

- [ ] **Step 9: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor: unify store error-to-HTTP status mapping

Add webutil.StoreHTTPStatus, WriteStoreErrorJSON, WriteStoreErrorText.
Remove writeStoreError from api/server.go and writeTagError from
api/handlers_tags.go. All store errors now mapped in one place.
Closes #34"
```

---

## Task 7: Service layer with store interfaces

**Files:**
- Create: `internal/service/interfaces.go`
- Create: `internal/service/inventory.go`
- Create: `internal/service/inventory_test.go`
- Create: `internal/service/bulk.go`
- Create: `internal/service/bulk_test.go`
- Create: `internal/service/tags.go`
- Create: `internal/service/tags_test.go`
- Create: `internal/service/search.go`
- Create: `internal/service/printers.go`
- Modify: `internal/api/server.go` — use services instead of store directly
- Modify: `internal/ui/server.go` — use services instead of store directly
- Modify: `internal/app/server.go` — create services and inject
- Modify: `cmd/qlx/main.go` — create services
- Modify: `CLAUDE.md` — update SaveOrFail pattern

This is the largest task. Implement incrementally: interfaces → bulk service (most duplicated) → tags → inventory → search + printers → wire handlers.

- [ ] **Step 1: Create `interfaces.go`**

Create `internal/service/interfaces.go` with all interfaces from the spec (ItemStore, ContainerStore, Saveable, TagStore, SearchStore, PrinterStore). See spec Step 6 for exact signatures.

- [ ] **Step 2: Write tests for bulk service**

Create `internal/service/bulk_test.go` with tests for BulkMove, BulkDelete, BulkTag using mock stores. Test both success and error paths.

- [ ] **Step 3: Run tests — expect FAIL**

```bash
go test ./internal/service/ -run TestBulk -v
```

- [ ] **Step 4: Implement `bulk.go`**

Create `internal/service/bulk.go` with BulkService. Extract logic from `internal/api/handlers_bulk.go` and `internal/ui/handlers_bulk.go`. The service owns validation, iteration, and Save().

- [ ] **Step 5: Run tests — expect PASS**

```bash
go test ./internal/service/ -run TestBulk -v
```

- [ ] **Step 6: Write tests for tag service**

Create `internal/service/tags_test.go`.

- [ ] **Step 7: Implement `tags.go`**

Create `internal/service/tags.go` with TagService.

- [ ] **Step 8: Run tag tests**

```bash
go test ./internal/service/ -run TestTag -v
```

- [ ] **Step 9: Write tests for inventory service**

Create `internal/service/inventory_test.go`.

- [ ] **Step 10: Implement `inventory.go`**

Create `internal/service/inventory.go` with InventoryService.

- [ ] **Step 11: Run inventory tests**

```bash
go test ./internal/service/ -run TestInventory -v
```

- [ ] **Step 12: Implement `search.go` and `printers.go`**

Thin services — mostly passthrough to store with Save() on mutations.

- [ ] **Step 13: Wire services into API and UI servers**

Update `internal/api/server.go`:
- Add service fields to Server struct
- Update NewServer to accept services
- Replace `s.store.Method()` calls with `s.inventory.Method()`, `s.bulk.Method()`, etc.
- Remove `SaveOrFail` calls from handlers (service handles Save)

Update `internal/ui/server.go`:
- Same pattern as API server

Update `internal/app/server.go`:
- Create services from store
- Pass to api.NewServer and ui.NewServer

- [ ] **Step 14: Update CLAUDE.md**

In `CLAUDE.md`, update the "Store Mutations — Always SaveOrFail" section:

```markdown
### Store Mutations

Service layer methods call `store.Save()` internally. Handlers check the returned error.
`webutil.SaveOrFail` is available for code not yet migrated to the service layer.
```

- [ ] **Step 15: Run all tests**

```bash
make test
make lint
make test-e2e
```

- [ ] **Step 16: Commit**

```bash
git add internal/ cmd/ CLAUDE.md Makefile tsconfig.json
git commit -m "refactor: introduce service layer with store interfaces

Add internal/service/ with InventoryService, BulkService, TagService,
SearchService, PrinterService. Define store interfaces for testability.
Handlers become thin adapters — parse request, call service, format response.
Save() moves into service layer.
Closes #38"
```

---

## Verification Checklist

After all tasks are complete:

- [ ] `make build-mac` succeeds
- [ ] `make lint` succeeds (including `tsc --noEmit`)
- [ ] `make test` succeeds
- [ ] `make test-e2e` succeeds
- [ ] All pages render identically to before
- [ ] Nav shows Polish strings by default
- [ ] Setting browser to English shows English nav
- [ ] Bulk select, drag & drop, move picker, tag picker, delete confirm all work
- [ ] SSE printer status updates work
- [ ] Template designer works
- [ ] No hardcoded Polish strings remain in JS
- [ ] All CSS uses design tokens (no hardcoded colors/spacing)
