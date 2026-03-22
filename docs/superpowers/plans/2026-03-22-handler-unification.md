# Handler Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge `internal/api/` and `internal/ui/` into a single `internal/handler/` package with content negotiation, eliminating handler duplication and enabling clean middleware (auth) composition.

**Architecture:** Single set of RESTful routes using `Responder` interface for content negotiation (JSON vs HTMX partial vs full HTML page). Domain handler structs implement `RouteRegistrar` interface. URL prefixes `/api/` and `/ui/` are dropped — format determined by `HX-Request` and `Accept` headers. New services (TemplateService, AssetService, ExportService) replace direct store access.

**Tech Stack:** Go 1.22+ `http.ServeMux`, HTMX 2.x (`HX-Request` header, `HX-Redirect`/`HX-Trigger` response headers, `Vary: HX-Request`), `html/template`

---

## File Structure

### New files to create

```
internal/handler/
  registrar.go              — RouteRegistrar interface
  responder.go              — Responder interface + JSONResponder + helper funcs
  responder_html.go         — HTMLResponder (template rendering, content negotiation)
  request.go                — BindRequest helper, shared request types
  viewmodels.go             — All view model types (moved from ui/server.go)
  containers.go             — ContainerHandler struct + routes
  items.go                  — ItemHandler struct + routes
  tags.go                   — TagHandler struct + routes
  bulk.go                   — BulkHandler struct + routes
  search.go                 — SearchHandler struct + routes
  print.go                  — PrintHandler struct + routes (printers, SSE, connect/disconnect)
  templates.go              — TemplateHandler struct + routes (designer, CRUD)
  assets.go                 — AssetHandler struct + routes (upload, serve)
  export.go                 — ExportHandler struct + routes (JSON, CSV)
  partials.go               — PartialsHandler struct + routes (tree, tag-tree)
  settings.go               — SettingsHandler struct + routes
  i18n.go                   — I18nHandler struct + routes
  bluetooth.go              — BluetoothHandler (build tag: ble)
  containers_test.go        — ContainerHandler unit tests
  tags_test.go              — TagHandler unit tests
  responder_test.go         — Responder unit tests

internal/service/
  templates.go              — TemplateService (new)
  assets.go                 — AssetService (new)
  export.go                 — ExportService (new)
  interfaces.go             — + TemplateStore, AssetStore, ExportStore, AllContainers

internal/shared/webutil/
  request.go                — BindRequest, IsJSONBody (moved from api/server.go)
```

### Files to delete

```
internal/api/               — entire package (replaced by handler/)
internal/ui/                — entire package (replaced by handler/)
```

### Files to modify

```
internal/app/server.go      — new composition root with RouteRegistrar wiring
internal/app/server_test.go — updated URLs (drop /api/ prefix)
internal/store/store.go     — + AllContainers(), template/asset store interface methods if missing
internal/embedded/static/js/ — update all fetch() URLs
internal/embedded/templates/ — update all hx-get/hx-post/href URLs
e2e/tests/                  — update all URL references
```

---

### Task 1: Responder interface + JSONResponder

**Files:**
- Create: `internal/handler/registrar.go`
- Create: `internal/handler/responder.go`
- Test: `internal/handler/responder_test.go`

- [ ] **Step 1: Write failing tests for JSONResponder**

```go
// internal/handler/responder_test.go
package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONResponder_Respond(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	data := map[string]string{"name": "test"}
	resp.Respond(w, r, http.StatusCreated, data, "", nil)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestJSONResponder_RespondError(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	resp.RespondError(w, r, errors.New("not found"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestJSONResponder_Redirect(t *testing.T) {
	resp := &JSONResponder{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	resp.Redirect(w, r, "/containers", map[string]bool{"ok": true})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestJSON -v`
Expected: FAIL — package/types don't exist yet

- [ ] **Step 3: Implement RouteRegistrar and Responder**

```go
// internal/handler/registrar.go
package handler

import "net/http"

// RouteRegistrar registers HTTP routes on a mux.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}
```

```go
// internal/handler/responder.go
package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// Responder handles content negotiation for HTTP responses.
type Responder interface {
	// Respond writes a response. For JSON: serializes data. For HTML: calls vmFn, renders tmpl.
	// vmFn may be nil for JSON-only endpoints.
	Respond(w http.ResponseWriter, r *http.Request, status int, data any, tmpl string, vmFn func() any)

	// RespondError writes an error response with content negotiation.
	RespondError(w http.ResponseWriter, r *http.Request, err error)

	// Redirect sends redirect. JSON: writes jsonData. HTMX: HX-Redirect. Browser: HTTP 303.
	Redirect(w http.ResponseWriter, r *http.Request, url string, jsonData any)
}

// JSONResponder always responds with JSON. Used in agent builds and for testing.
type JSONResponder struct{}

func (j *JSONResponder) Respond(w http.ResponseWriter, r *http.Request, status int, data any, _ string, _ func() any) {
	webutil.JSON(w, status, data)
}

func (j *JSONResponder) RespondError(w http.ResponseWriter, r *http.Request, err error) {
	webutil.WriteStoreErrorJSON(w, err)
}

func (j *JSONResponder) Redirect(w http.ResponseWriter, r *http.Request, _ string, jsonData any) {
	webutil.JSON(w, http.StatusOK, jsonData)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestJSON -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/registrar.go internal/handler/responder.go internal/handler/responder_test.go
git commit -m "feat(handler): add RouteRegistrar and Responder interfaces with JSONResponder"
```

---

### Task 2: HTMLResponder (content negotiation)

**Files:**
- Create: `internal/handler/responder_html.go`
- Modify: `internal/handler/responder_test.go`

- [ ] **Step 1: Write failing tests for HTMLResponder**

```go
// Add to responder_test.go
func TestHTMLResponder_JSON_WhenAcceptJSON(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "application/json")

	data := map[string]string{"name": "test"}
	resp.Respond(w, r, http.StatusOK, data, "containers", func() any { return nil })

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected JSON content type, got %s", ct)
	}
}

func TestHTMLResponder_Partial_WhenHTMX(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("HX-Request", "true")

	vmCalled := false
	resp.Respond(w, r, http.StatusOK, nil, "containers", func() any {
		vmCalled = true
		return nil // template execution will fail — we just check vmFn was called
	})

	if !vmCalled {
		t.Fatal("expected vmFn to be called for HTMX request")
	}
}

func TestHTMLResponder_Redirect_HTMX(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("HX-Request", "true")

	resp.Redirect(w, r, "/containers", nil)

	if w.Header().Get("HX-Redirect") != "/containers" {
		t.Fatalf("expected HX-Redirect header, got %q", w.Header().Get("HX-Redirect"))
	}
}

func TestHTMLResponder_Redirect_Browser(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)

	resp.Redirect(w, r, "/containers", nil)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestHTMLResponder_Redirect_JSON(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Accept", "application/json")

	resp.Redirect(w, r, "/containers", map[string]bool{"ok": true})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTMLResponder_VaryHeader(t *testing.T) {
	resp := newTestHTMLResponder(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "application/json")

	resp.Respond(w, r, http.StatusOK, nil, "", nil)

	if v := w.Header().Get("Vary"); v != "HX-Request" {
		t.Fatalf("expected Vary: HX-Request, got %q", v)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestHTMLResponder -v`
Expected: FAIL

- [ ] **Step 3: Implement HTMLResponder**

```go
// internal/handler/responder_html.go
package handler

import (
	"html/template"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// HTMLResponder negotiates response format based on request headers.
// HX-Request → HTML partial, Accept: application/json → JSON, otherwise → full HTML page.
type HTMLResponder struct {
	templates    map[string]*template.Template
	translations *webutil.Translations
}

// NewHTMLResponder creates a responder with template rendering support.
func NewHTMLResponder(templates map[string]*template.Template, translations *webutil.Translations) *HTMLResponder {
	return &HTMLResponder{templates: templates, translations: translations}
}

func (h *HTMLResponder) Respond(w http.ResponseWriter, r *http.Request, status int, data any, tmpl string, vmFn func() any) {
	w.Header().Set("Vary", "HX-Request")

	if webutil.WantsJSON(r) {
		webutil.JSON(w, status, data)
		return
	}

	if vmFn == nil {
		webutil.JSON(w, status, data)
		return
	}

	vm := vmFn()
	t, ok := h.templates[tmpl]
	if !ok {
		http.Error(w, "template not found: "+tmpl, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	page := PageData{
		Lang:       langFromRequest(r),
		translator: h.translations,
		Data:       vm,
	}

	templateName := tmpl
	if !webutil.IsHTMX(r) {
		templateName = "layout"
	}

	if err := t.ExecuteTemplate(w, templateName, page); err != nil {
		webutil.LogError("template execute: %v", err)
	}
}

func (h *HTMLResponder) RespondError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Vary", "HX-Request")

	if webutil.WantsJSON(r) {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}

	status := webutil.StoreHTTPStatus(err)
	http.Error(w, err.Error(), status)
}

func (h *HTMLResponder) Redirect(w http.ResponseWriter, r *http.Request, url string, jsonData any) {
	if webutil.WantsJSON(r) {
		webutil.JSON(w, http.StatusOK, jsonData)
		return
	}

	if webutil.IsHTMX(r) {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, url, http.StatusSeeOther)
}

func langFromRequest(r *http.Request) string {
	if v := r.Context().Value(webutil.LangKey); v != nil {
		return v.(string)
	}
	return "pl"
}
```

- [ ] **Step 4: Verify `IsJSON` exists in webutil, add `WantsJSON` alias**

Check `internal/shared/webutil/response.go` — `IsJSON(r)` already checks `Accept: application/json`.
Either rename `IsJSON` → `WantsJSON` (preferred — clearer name) or add alias:
```go
// WantsJSON returns true if the client prefers JSON responses.
var WantsJSON = IsJSON
```
Use `WantsJSON` consistently in the new handler code.

- [ ] **Step 5: Add test helper and run tests**

```go
func newTestHTMLResponder(t *testing.T) *HTMLResponder {
	t.Helper()
	// Minimal template for testing
	tmpl := template.Must(template.New("containers").Parse(`{{define "containers"}}test{{end}}{{define "layout"}}layout{{end}}`))
	return NewHTMLResponder(
		map[string]*template.Template{"containers": tmpl},
		webutil.NewTranslations(),
	)
}
```

Run: `go test ./internal/handler/ -run TestHTMLResponder -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handler/responder_html.go internal/handler/responder_test.go internal/shared/webutil/response.go
git commit -m "feat(handler): add HTMLResponder with HX-Request/Accept content negotiation"
```

---

### Task 3: BindRequest helper + shared request types

**Files:**
- Create: `internal/handler/request.go`
- Test: `internal/handler/request_test.go`

- [ ] **Step 1: Write failing tests for BindRequest**

```go
// internal/handler/request_test.go
package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindRequest_FormValues(t *testing.T) {
	var req struct {
		Name  string `json:"name" form:"name"`
		Color string `json:"color" form:"color"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader("name=Test&color=red"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Test" {
		t.Fatalf("expected Test, got %s", req.Name)
	}
}

func TestBindRequest_JSONBody(t *testing.T) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader(`{"name":"Test","color":"blue"}`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Test" {
		t.Fatalf("expected Test, got %s", req.Name)
	}
}

func TestBindRequest_JSONOverridesForm(t *testing.T) {
	var req struct {
		Name string `json:"name" form:"name"`
	}
	r := httptest.NewRequest("POST", "/?name=FromForm",
		strings.NewReader(`{"name":"FromJSON"}`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "FromJSON" {
		t.Fatalf("expected FromJSON, got %s", req.Name)
	}
}

func TestBindRequest_InvalidJSON(t *testing.T) {
	var req struct{ Name string }
	r := httptest.NewRequest("POST", "/",
		strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestBindRequest -v`

- [ ] **Step 3: Implement BindRequest**

```go
// internal/handler/request.go
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// BindRequest populates req from form values first, then overrides with JSON body
// if Content-Type is application/json. Uses `form` struct tags for form field mapping,
// falling back to `json` tags.
func BindRequest(r *http.Request, req any) error {
	_ = r.ParseForm()

	v := reflect.ValueOf(req).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		formTag := field.Tag.Get("form")
		if formTag == "" {
			formTag = field.Tag.Get("json")
		}
		if formTag == "" || formTag == "-" {
			continue
		}
		// Strip json options like ",omitempty"
		formTag = strings.Split(formTag, ",")[0]

		if val := r.FormValue(formTag); val != "" {
			fv := v.Field(i)
			switch fv.Kind() {
			case reflect.String:
				fv.SetString(val)
			case reflect.Int, reflect.Int64:
				if n, err := strconv.Atoi(val); err == nil {
					fv.SetInt(int64(n))
				}
			}
		}
	}

	if isJSONBody(r) {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	}

	return nil
}

// isJSONBody checks if the request body is JSON.
func isJSONBody(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/handler/ -run TestBindRequest -v`
Expected: PASS

- [ ] **Step 5: Add shared request types**

```go
// Add to request.go — shared request types used across handlers

// CreateContainerRequest is the input for container creation.
type CreateContainerRequest struct {
	ParentID    string `json:"parent_id" form:"parent_id"`
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// UpdateContainerRequest is the input for container updates.
type UpdateContainerRequest struct {
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// CreateItemRequest is the input for item creation.
type CreateItemRequest struct {
	ContainerID string `json:"container_id" form:"container_id"`
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Quantity    int    `json:"quantity"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// UpdateItemRequest is the input for item updates.
type UpdateItemRequest struct {
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Quantity    int    `json:"quantity"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// MoveRequest is the input for move operations.
type MoveRequest struct {
	ParentID    string `json:"parent_id" form:"parent_id"`
	ContainerID string `json:"container_id" form:"container_id"`
}

// UpsertTagRequest is the input for tag create/update.
type UpsertTagRequest struct {
	Name     string `json:"name" form:"name"`
	ParentID string `json:"parent_id" form:"parent_id"`
	Color    string `json:"color" form:"color"`
	Icon     string `json:"icon" form:"icon"`
}

// AddPrinterRequest is the input for adding a printer.
type AddPrinterRequest struct {
	Name      string `json:"name" form:"name"`
	Encoder   string `json:"encoder" form:"encoder"`
	Model     string `json:"model" form:"model"`
	Transport string `json:"transport" form:"transport"`
	Address   string `json:"address" form:"address"`
}

// PrintRequest is the input for print operations.
type PrintRequest struct {
	PrinterID string `json:"printer_id"`
	Template  string `json:"template"`
}

// TagAssignRequest is the input for assigning/removing tags.
type TagAssignRequest struct {
	TagID string `json:"tag_id" form:"tag_id"`
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/request.go internal/handler/request_test.go
git commit -m "feat(handler): add BindRequest helper and shared request types"
```

---

### Task 4: New services (TemplateService, AssetService, ExportService)

**Files:**
- Create: `internal/service/templates.go`
- Create: `internal/service/assets.go`
- Create: `internal/service/export.go`
- Modify: `internal/service/interfaces.go`
- Modify: `internal/service/inventory.go` (add AllContainers)
- Test: `internal/service/templates_test.go`

- [ ] **Step 1: Add new store interfaces**

Add to `internal/service/interfaces.go`:
```go
// TemplateStore defines template-related store operations.
type TemplateStore interface {
	AllTemplates() []store.Template
	GetTemplate(id string) *store.Template
	CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) *store.Template
	SaveTemplate(t store.Template)
	DeleteTemplate(id string)
}

// AssetStore defines asset-related store operations.
type AssetStore interface {
	SaveAsset(name, mimeType string, data []byte) (*store.Asset, error)
	GetAsset(id string) *store.Asset
	AssetData(id string) ([]byte, error)
}

// ExportStore defines export-related store operations.
type ExportStore interface {
	ExportData() (map[string]*store.Container, map[string]*store.Item)
	AllItems() []store.Item
	AllContainers() []store.Container
}
```

- [ ] **Step 2: Implement TemplateService**

```go
// internal/service/templates.go
package service

import "github.com/erxyi/qlx/internal/store"

// TemplateService manages label template operations.
type TemplateService struct {
	store interface {
		TemplateStore
		Saveable
	}
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(s interface {
	TemplateStore
	Saveable
}) *TemplateService {
	return &TemplateService{store: s}
}

// AllTemplates returns all templates.
func (s *TemplateService) AllTemplates() []store.Template {
	return s.store.AllTemplates()
}

// GetTemplate returns a template by ID or nil.
func (s *TemplateService) GetTemplate(id string) *store.Template {
	return s.store.GetTemplate(id)
}

// CreateTemplate creates a new template and persists it.
func (s *TemplateService) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	t := s.store.CreateTemplate(name, tags, target, widthMM, heightMM, widthPx, heightPx, elements)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return t, nil
}

// SaveTemplate updates an existing template and persists it.
func (s *TemplateService) SaveTemplate(t store.Template) error {
	s.store.SaveTemplate(t)
	return s.store.Save()
}

// DeleteTemplate deletes a template and persists the change.
func (s *TemplateService) DeleteTemplate(id string) error {
	s.store.DeleteTemplate(id)
	return s.store.Save()
}
```

- [ ] **Step 3: Implement AssetService**

```go
// internal/service/assets.go
package service

import "github.com/erxyi/qlx/internal/store"

// AssetService manages asset (image) operations.
type AssetService struct {
	store interface {
		AssetStore
		Saveable
	}
}

// NewAssetService creates a new AssetService.
func NewAssetService(s interface {
	AssetStore
	Saveable
}) *AssetService {
	return &AssetService{store: s}
}

// SaveAsset stores an asset and persists the change.
func (s *AssetService) SaveAsset(name, mimeType string, data []byte) (*store.Asset, error) {
	asset, err := s.store.SaveAsset(name, mimeType, data)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return asset, nil
}

// GetAsset returns asset metadata by ID.
func (s *AssetService) GetAsset(id string) *store.Asset {
	return s.store.GetAsset(id)
}

// AssetData returns the raw asset data by ID.
func (s *AssetService) AssetData(id string) ([]byte, error) {
	return s.store.AssetData(id)
}
```

- [ ] **Step 4: Implement ExportService**

```go
// internal/service/export.go
package service

import "github.com/erxyi/qlx/internal/store"

// ExportService handles data export operations.
type ExportService struct {
	store     ExportStore
	inventory *InventoryService
}

// NewExportService creates a new ExportService.
func NewExportService(s ExportStore, inventory *InventoryService) *ExportService {
	return &ExportService{store: s, inventory: inventory}
}

// ExportJSON returns the full data export payload.
func (s *ExportService) ExportJSON() (map[string]*store.Container, map[string]*store.Item) {
	return s.store.ExportData()
}

// AllItems returns all items (for CSV export).
func (s *ExportService) AllItems() []store.Item {
	return s.store.AllItems()
}

// AllContainers returns all containers.
func (s *ExportService) AllContainers() []store.Container {
	return s.store.AllContainers()
}
```

- [ ] **Step 5: Add ErrNotFound sentinel error to service package**

Add to `internal/service/interfaces.go`:
```go
import "errors"

// ErrNotFound is a generic "not found" error for use in handlers.
var ErrNotFound = errors.New("not found")
```

- [ ] **Step 6: Add AllContainers to InventoryService**

Add to `internal/service/inventory.go`:
```go
// AllContainers returns all containers without filtering.
func (s *InventoryService) AllContainers() []store.Container {
	return s.store.AllContainers()
}
```

Ensure `ContainerStore` interface includes `AllContainers() []store.Container` — add if missing.

- [ ] **Step 7: Write basic tests for TemplateService**

```go
// internal/service/templates_test.go
package service

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestTemplateService_CreateTemplate(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewTemplateService(s)

	tmpl, err := svc.CreateTemplate("Test", "", "ql700", 62, 29, 720, 320, "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Name != "Test" {
		t.Fatalf("expected Test, got %s", tmpl.Name)
	}

	all := svc.AllTemplates()
	if len(all) != 1 {
		t.Fatalf("expected 1 template, got %d", len(all))
	}
}

func TestTemplateService_DeleteTemplate(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewTemplateService(s)

	tmpl, _ := svc.CreateTemplate("ToDelete", "", "ql700", 62, 29, 720, 320, "{}")
	if err := svc.DeleteTemplate(tmpl.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.AllTemplates()) != 0 {
		t.Fatal("expected 0 templates after delete")
	}
}
```

- [ ] **Step 8: Run tests**

Run: `go test ./internal/service/ -v`
Expected: PASS (may need store method adjustments — fix as needed)

- [ ] **Step 9: Commit**

```bash
git add internal/service/templates.go internal/service/assets.go internal/service/export.go internal/service/interfaces.go internal/service/inventory.go internal/service/templates_test.go
git commit -m "feat(service): add TemplateService, AssetService, ExportService and AllContainers"
```

---

### Task 5: View models + PageData (move from ui/)

**Files:**
- Create: `internal/handler/viewmodels.go`

- [ ] **Step 1: Move all view model types from ui/server.go**

```go
// internal/handler/viewmodels.go
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
}

// ItemDetailData is the view model for the item detail page.
type ItemDetailData struct {
	Item      *store.Item
	Path      []store.Container
	Printers  []store.PrinterConfig
	Templates []store.Template
	Schemas   []string
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

// SettingsData is the view model for the settings page (currently empty).
type SettingsData struct{}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/handler/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/handler/viewmodels.go
git commit -m "feat(handler): add view model types (moved from ui)"
```

---

### Task 6: ContainerHandler — first domain handler

**Files:**
- Create: `internal/handler/containers.go`
- Test: `internal/handler/containers_test.go`

This is the template for all other domain handlers. Get this right and the rest follow the same pattern.

- [ ] **Step 1: Write failing test for container list**

```go
// internal/handler/containers_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

func TestContainerHandler_List_JSON(t *testing.T) {
	s := store.NewMemoryStore()
	inv := service.NewInventoryService(s)
	inv.CreateContainer("", "Box1", "", "", "")

	h := &ContainerHandler{
		inventory: inv,
		resp:      &JSONResponder{},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/containers", nil)
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Box1") {
		t.Fatalf("expected Box1 in response, got %s", w.Body.String())
	}
}

func TestContainerHandler_Create_JSON(t *testing.T) {
	s := store.NewMemoryStore()
	inv := service.NewInventoryService(s)

	h := &ContainerHandler{
		inventory: inv,
		resp:      &JSONResponder{},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/containers",
		strings.NewReader(`{"name":"NewBox"}`))
	r.Header.Set("Content-Type", "application/json")
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "NewBox") {
		t.Fatalf("expected NewBox in response, got %s", w.Body.String())
	}
}

func TestContainerHandler_Delete_JSON(t *testing.T) {
	s := store.NewMemoryStore()
	inv := service.NewInventoryService(s)
	c := inv.CreateContainer("", "ToDelete", "", "", "")

	h := &ContainerHandler{
		inventory: inv,
		resp:      &JSONResponder{},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/containers/"+c.ID, nil)
	r.SetPathValue("id", c.ID)
	h.Delete(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestContainerHandler -v`

- [ ] **Step 3: Implement ContainerHandler**

```go
// internal/handler/containers.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
)

// ContainerHandler handles container-related HTTP routes.
type ContainerHandler struct {
	inventory *service.InventoryService
	templates *service.TemplateService
	printers  *service.PrinterService
	resp      Responder
}

// NewContainerHandler creates a new ContainerHandler.
func NewContainerHandler(inv *service.InventoryService, tmpl *service.TemplateService, prn *service.PrinterService, resp Responder) *ContainerHandler {
	return &ContainerHandler{inventory: inv, templates: tmpl, printers: prn, resp: resp}
}

// RegisterRoutes registers container routes on the mux.
func (h *ContainerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /containers", h.List)
	mux.HandleFunc("GET /containers/{id}", h.Detail)
	mux.HandleFunc("POST /containers", h.Create)
	mux.HandleFunc("PUT /containers/{id}", h.Update)
	mux.HandleFunc("DELETE /containers/{id}", h.Delete)
	mux.HandleFunc("GET /containers/{id}/items", h.Items)
	mux.HandleFunc("GET /containers/{id}/items-json", h.ItemsJSON) // used by print batch UI
	mux.HandleFunc("PATCH /containers/{id}/move", h.Move)
	mux.HandleFunc("GET /containers/{id}/edit", h.Edit)
}

// List returns containers (optionally filtered by parent_id).
func (h *ContainerHandler) List(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")

	var containers any
	if parentID == "" {
		containers = h.inventory.AllContainers()
	} else {
		containers = h.inventory.ContainerChildren(parentID)
	}

	h.resp.Respond(w, r, http.StatusOK, containers, "containers", func() any {
		return h.containerListVM(parentID)
	})
}

// Detail returns a single container with its children and path.
func (h *ContainerHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, service.ErrNotFound)
		return
	}

	data := map[string]any{
		"container": container,
		"children":  h.inventory.ContainerChildren(id),
		"path":      h.inventory.ContainerPath(id),
	}

	h.resp.Respond(w, r, http.StatusOK, data, "containers", func() any {
		return h.containerListVM(id)
	})
}

// Create creates a new container.
func (h *ContainerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateContainerRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container, err := h.inventory.CreateContainer(req.ParentID, req.Name, req.Description, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusCreated, container, "containers", func() any {
		return h.containerListVM(req.ParentID)
	})
}

// Update updates a container.
func (h *ContainerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateContainerRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container, err := h.inventory.UpdateContainer(id, req.Name, req.Description, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, container, "containers", func() any {
		return h.containerListVM(container.ParentID)
	})
}

// Delete deletes a container.
func (h *ContainerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, service.ErrNotFound)
		return
	}

	parentID := container.ParentID
	if err := h.inventory.DeleteContainer(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]bool{"ok": true}, "containers", func() any {
		return h.containerListVM(parentID)
	})
}

// Items returns items in a container.
func (h *ContainerHandler) Items(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, service.ErrNotFound)
		return
	}

	items := h.inventory.ContainerItems(id)
	data := map[string]any{
		"items": items,
		"path":  h.inventory.ContainerPath(id),
	}

	h.resp.Respond(w, r, http.StatusOK, data, "containers", func() any {
		return h.containerListVM(id)
	})
}

// Move moves a container to a new parent.
func (h *ContainerHandler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req MoveRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.inventory.MoveContainer(id, req.ParentID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]bool{"ok": true}, "", nil)
}

// Edit renders the container edit form.
func (h *ContainerHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, service.ErrNotFound)
		return
	}

	data := map[string]any{"container": container}
	h.resp.Respond(w, r, http.StatusOK, data, "container-form", func() any {
		return ContainerFormData{
			Container: container,
			Path:      h.inventory.ContainerPath(id),
		}
	})
}

// containerListVM builds the full view model for the container list page.
func (h *ContainerHandler) containerListVM(parentID string) ContainerListData {
	vm := ContainerListData{
		Children:  h.inventory.ContainerChildren(parentID),
		Printers:  h.safeAllPrinters(),
		Templates: h.safeAllTemplates(),
		Schemas:   label.SchemaNames(),
	}

	if parentID != "" {
		container := h.inventory.GetContainer(parentID)
		if container != nil {
			vm.Container = container
			vm.Items = h.inventory.ContainerItems(parentID)
			vm.Path = h.inventory.ContainerPath(parentID)
		}
	}

	return vm
}

func (h *ContainerHandler) safeAllPrinters() []store.PrinterConfig {
	if h.printers == nil {
		return nil
	}
	return h.printers.AllPrinters()
}

func (h *ContainerHandler) safeAllTemplates() []store.Template {
	if h.templates == nil {
		return nil
	}
	return h.templates.AllTemplates()
}

// parseQuantity parses a quantity string with a default value.
func parseQuantity(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	if q, err := strconv.Atoi(s); err == nil {
		return q
	}
	return defaultVal
}
```

Note: Import `store` package for `store.PrinterConfig`, `store.Template` types. The `safeAll*` methods handle nil services gracefully (for agent build where templates/printers may not exist).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/handler/ -run TestContainerHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/containers.go internal/handler/containers_test.go
git commit -m "feat(handler): add ContainerHandler with unified routes and content negotiation"
```

---

### Task 7: Remaining domain handlers

Each follows the ContainerHandler pattern. Implement one at a time with tests.

**Files per handler:**
- Create: `internal/handler/<domain>.go`
- Test: `internal/handler/<domain>_test.go` (at minimum: one JSON create, one JSON list or detail)

#### 7a: ItemHandler

- [ ] **Step 1: Implement ItemHandler**

Key differences from ContainerHandler:
- `Create` uses `parseQuantity(r.FormValue("quantity"), 1)` for form submissions
- `Detail` builds `ItemDetailData` view model
- Routes: `GET /items/{id}`, `POST /items`, `PUT /items/{id}`, `DELETE /items/{id}`, `PATCH /items/{id}/move`, `GET /items/{id}/edit`
- Label data construction: `QRContent: "/items/" + item.ID` (fix the QR URL inconsistency!)

- [ ] **Step 2: Write at least 2 tests (Create + Detail)**
- [ ] **Step 3: Run tests, verify pass**
- [ ] **Step 4: Commit**: `feat(handler): add ItemHandler`

#### 7b: TagHandler

- [ ] **Step 1: Implement TagHandler**

Key points:
- Routes: `GET /tags`, `POST /tags`, `GET /tags/{id}`, `PUT /tags/{id}`, `DELETE /tags/{id}`, `PATCH /tags/{id}/move`, `GET /tags/{id}/descendants`
- Tag assignment routes: `POST /items/{id}/tags`, `DELETE /items/{id}/tags/{tag_id}`, `POST /containers/{id}/tags`, `DELETE /containers/{id}/tags/{tag_id}`
- `resolveTagIDs` method — defined once here, also registered as template FuncMap
- Tag assignment handlers: generalize item/container add/remove into shared logic with `objectType` param
- Fix: tag create error should use `RespondError` (maps to correct status), not hardcoded 500
- Delete uses `Redirect` for HTML, JSON data for API

- [ ] **Step 2: Write tests (Create + tag assignment)**
- [ ] **Step 3: Run tests, verify pass**
- [ ] **Step 4: Commit**: `feat(handler): add TagHandler with unified tag assignment`

#### 7c: BulkHandler

- [ ] **Step 1: Implement BulkHandler**

Key points:
- Routes: `POST /bulk/move`, `POST /bulk/delete`, `POST /bulk/tags`
- Always JSON response (even from HTMX) — use `JSONResponder` directly or just `webutil.JSON`
- Standardize response shape: `{"ok": bool, "errors": [...]}` (merge api + ui shapes)

- [ ] **Step 2: Write test**
- [ ] **Step 3: Commit**: `feat(handler): add BulkHandler`

#### 7d: SearchHandler

- [ ] **Step 1: Implement SearchHandler**

Key points:
- Routes: `GET /search`
- JSON: returns `{containers, items, tags}`, HTML: renders search results page
- Single handler replaces both api and ui search

- [ ] **Step 2: Write test**
- [ ] **Step 3: Commit**: `feat(handler): add SearchHandler`

#### 7e: PrintHandler

- [ ] **Step 1: Implement PrintHandler**

Key points:
- Routes: `GET /printers`, `POST /printers`, `DELETE /printers/{id}`, `GET /encoders`, `POST /items/{id}/print`, `POST /print-image`, `GET /printers/status`, `GET /printers/{id}/status`, `POST /printers/{id}/connect`, `POST /printers/{id}/disconnect`, `GET /printers/events` (SSE)
- **NOTE:** `POST /print-image` must be explicitly registered — it was a separate route in ui (`/ui/actions/print-image`)
- Printer status/connect/disconnect use `printerManager` directly — this is fine
- SSE handler is unique — no content negotiation, always SSE
- Print handler builds `label.LabelData` — deduplicate into shared helper
- Printers list page: `Respond` with `PrintersData` view model

- [ ] **Step 2: Write test (printer create + list)**
- [ ] **Step 3: Commit**: `feat(handler): add PrintHandler with SSE and printer management`

#### 7f: TemplateHandler

- [ ] **Step 1: Implement TemplateHandler**

Key points:
- Routes: `GET /templates`, `GET /templates/new`, `GET /templates/{id}/edit`, `POST /templates`, `PUT /templates/{id}`, `DELETE /templates/{id}`
- Uses `TemplateService` (new) — no direct store access
- Deduplicate preview sample data into a constant
- Template designer is always full HTML page (no JSON variant)
- Template save is always JSON response

- [ ] **Step 2: Write test**
- [ ] **Step 3: Commit**: `feat(handler): add TemplateHandler using TemplateService`

#### 7g: AssetHandler

- [ ] **Step 1: Implement AssetHandler**

Key points:
- Routes: `POST /assets`, `GET /assets/{id}`
- Uses `AssetService` (new) — no direct store access
- Upload: multipart form, returns JSON
- Serve: binary response with content type from asset metadata

- [ ] **Step 2: Commit**: `feat(handler): add AssetHandler using AssetService`

#### 7h: ExportHandler

- [ ] **Step 1: Implement ExportHandler**

Key points:
- Routes: `GET /export/json`, `GET /export/csv`
- Uses `ExportService` (new) — no direct store access
- Always specific content type (JSON / CSV) — no content negotiation

- [ ] **Step 2: Commit**: `feat(handler): add ExportHandler using ExportService`

#### 7i: PartialsHandler

- [ ] **Step 1: Implement PartialsHandler**

Key points:
- Routes: `GET /partials/tree`, `GET /partials/tree/search`, `GET /partials/tag-tree`, `GET /partials/tag-tree/search`
- Always HTML partial response — uses `renderPartial` directly on HTMLResponder
- Need to expose `RenderPartial` method on Responder interface or handle directly

Design decision: PartialsHandler uses HTMLResponder's `RenderPartial` method directly (type assertion or separate interface). These are inherently HTML-only endpoints. If Responder is JSONResponder (agent build), these routes simply aren't registered.

- [ ] **Step 2: Commit**: `feat(handler): add PartialsHandler for tree partials`

#### 7j: SettingsHandler + I18nHandler

- [ ] **Step 1: Implement SettingsHandler**

- Routes: `GET /settings`, `POST /set-lang`
- Settings: HTML page only. Set-lang: cookie + redirect.

- [ ] **Step 2: Implement I18nHandler**

- Route: `GET /i18n/{lang}`
- Always JSON response

- [ ] **Step 3: Commit**: `feat(handler): add SettingsHandler and I18nHandler`

#### 7k: BluetoothHandler

- [ ] **Step 1: Implement BluetoothHandler (build tag: ble)**

- Route: `GET /bluetooth/scan`
- Same as current, just moved to handler package

- [ ] **Step 2: Commit**: `feat(handler): add BluetoothHandler (ble build tag)`

---

### Task 8: Template loading (move from ui/server.go)

**Files:**
- Create: `internal/handler/templates_load.go`

- [ ] **Step 1: Move template loading functions**

Move from `ui/server.go` to `internal/handler/templates_load.go`:
- `loadTemplates()`
- `loadLayout()`
- `mergeHTMLDir()`
- `discoverPages()`
- `dict()` template func

Keep the same logic. The `resolveTagsFn` will be passed from the composition root (TagService.GetTag).

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/handler/`

- [ ] **Step 3: Commit**

```bash
git commit -m "refactor(handler): move template loading from ui to handler package"
```

---

### Task 9: New composition root

**Files:**
- Modify: `internal/app/server.go`
- Modify: `internal/app/server_test.go`

- [ ] **Step 1: Rewrite app/server.go**

```go
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

type Server struct {
	handler http.Handler
}

func NewServer(s *store.Store, pm *qlprint.PrinterManager) *Server {
	translations := webutil.NewTranslations()
	if err := translations.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		panic(err)
	}

	// Services
	inventory := service.NewInventoryService(s)
	bulk := service.NewBulkService(s)
	tags := service.NewTagService(s)
	search := service.NewSearchService(s)
	printers := service.NewPrinterService(s)
	templates := service.NewTemplateService(s)
	assets := service.NewAssetService(s)
	export := service.NewExportService(s, inventory)

	// Responder with template rendering
	resolveTagsFn := func(ids []string) []store.Tag {
		result := make([]store.Tag, 0, len(ids))
		for _, id := range ids {
			if t := tags.GetTag(id); t != nil {
				result = append(result, *t)
			}
		}
		return result
	}
	tmplMap := handler.LoadTemplates(resolveTagsFn)
	resp := handler.NewHTMLResponder(tmplMap, translations)

	// Domain handlers
	registrars := []handler.RouteRegistrar{
		handler.NewContainerHandler(inventory, templates, printers, resp),
		handler.NewItemHandler(inventory, templates, printers, resp),
		handler.NewTagHandler(tags, inventory, resp),
		handler.NewBulkHandler(bulk),
		handler.NewSearchHandler(search, resp),
		handler.NewPrintHandler(pm, inventory, printers, templates, resp),
		handler.NewTemplateHandler(templates, pm, resp),
		handler.NewAssetHandler(assets),
		handler.NewExportHandler(export, inventory),
		handler.NewPartialsHandler(inventory, search, tags, resp),
		handler.NewSettingsHandler(resp),
		handler.NewI18nHandler(translations),
	}

	mux := http.NewServeMux()

	// Static files
	staticFS, _ := fs.Sub(embedded.Static, "static")
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
		w.Write(data)
	})

	// Root redirects to container list
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// Delegate to container handler list (root = empty parent)
		registrars[0].(*handler.ContainerHandler).List(w, r)
	})

	// Register all domain routes
	for _, reg := range registrars {
		reg.RegisterRoutes(mux)
	}

	return &Server{handler: webutil.LangMiddleware("pl")(webutil.LoggingMiddleware(mux))}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
```

- [ ] **Step 2: Update server_test.go URLs**

Replace all `/api/containers` → `/containers`, `/api/items` → `/items`, etc. in test requests.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/app/ -v`

- [ ] **Step 4: Commit**

```bash
git commit -m "refactor(app): rewrite composition root with unified handler modules"
```

---

### Task 10: Update frontend URLs — templates

**Files:**
- Modify: All files in `internal/embedded/templates/`

- [ ] **Step 1: Global URL replacements in HTML templates**

Apply these replacements across all template files:

| Old | New |
|-----|-----|
| `/ui/actions/containers` | `/containers` |
| `/ui/actions/items` | `/items` |
| `/ui/actions/printers` | `/printers` |
| `/ui/actions/templates` | `/templates` |
| `/ui/actions/tags` | `/tags` |
| `/ui/actions/bulk/move` | `/bulk/move` |
| `/ui/actions/bulk/delete` | `/bulk/delete` |
| `/ui/actions/bulk/tags` | `/bulk/tags` |
| `/ui/actions/set-lang` | `/set-lang` |
| `/ui/actions/items/{id}/tags` | `/items/{id}/tags` |
| `/ui/actions/containers/{id}/tags` | `/containers/{id}/tags` |
| `/ui/actions/print-image` | `/print-image` |
| `/ui/actions/assets` | `/assets` |
| `/ui/containers/` | `/containers/` |
| `/ui/items/` | `/items/` |
| `/ui/printers` | `/printers` |
| `/ui/templates` | `/templates` |
| `/ui/tags` | `/tags` |
| `/ui/search` | `/search` |
| `/ui/settings` | `/settings` |
| `/ui/partials/` | `/partials/` |
| `href="/ui"` | `href="/"` |
| `hx-get="/ui"` | `hx-get="/"` |

Note: Be careful with partial matches. Do replacements from longest to shortest to avoid double-replacement.

- [ ] **Step 2: Verify no remaining /ui/ or /api/ references in templates**

Run: `grep -r '/ui/' internal/embedded/templates/ && grep -r '/api/' internal/embedded/templates/`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git commit -m "refactor(templates): update all URLs to use unified route scheme"
```

---

### Task 11: Update frontend URLs — JavaScript

**Files:**
- Modify: All files in `internal/embedded/static/js/` and `internal/embedded/static/label-*.js`

- [ ] **Step 1: Update JS fetch URLs**

| Old | New |
|-----|-----|
| `/ui/actions/bulk/delete` | `/bulk/delete` |
| `/ui/actions/bulk/move` | `/bulk/move` |
| `/ui/actions/bulk/tags` | `/bulk/tags` |
| `/ui/actions/assets` | `/assets` |
| `/ui/actions/print-image` | `/print-image` |
| `/api/i18n/` | `/i18n/` |
| `/api/printers/status` | `/printers/status` |
| `/api/printers/events` | `/printers/events` |
| `/api/tags` | `/tags` |
| `/ui/actions/` (in tag JS dynamic URLs) | `/` |
| `/ui/` (in tag JS return URLs) | `/` |
| `/ui/actions/assets/` (in qlx-format.js) | `/assets/` |

Also check for `htmx.ajax()` calls in `label-designer.js` (references `/ui/templates`).

For JS `fetch()` calls that expect JSON, ensure they set `Accept: application/json` header. Check each call:
- `sse.js`: `fetch("/printers/status")` — expects JSON → add `headers: {"Accept": "application/json"}`
- `i18n.js`: `fetch("/i18n/...")` — always JSON endpoint, no change needed
- `dragdrop.js`: move URL construction — check if it needs Accept header
- `label-designer.js`: asset upload — multipart, response is JSON → add Accept header
- `label-print.js`: print image — expects JSON → add Accept header
- `tree-picker.js`: loads tree data — check if HTML partial or JSON

- [ ] **Step 2: Verify no remaining old URLs**

Run: `grep -r '/ui/actions\|/api/' internal/embedded/static/`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git commit -m "refactor(js): update all fetch URLs to unified route scheme"
```

---

### Task 12: Update E2E tests

**Files:**
- Modify: All files in `e2e/tests/`

- [ ] **Step 1: Update E2E test URLs**

Replace all URL patterns:
- `/api/containers` → `/containers`
- `/api/items` → `/items`
- `/api/tags` → `/tags`
- `/api/bulk/` → `/bulk/`
- `/api/search` → `/search`
- `/ui/actions/` → remove prefix (adjust path)
- `/ui/containers/` → `/containers/`
- `/ui/items/` → `/items/`
- `/ui/templates` → `/templates`
- `/ui/search` → `/search`
- `/ui/tags` → `/tags`

For Playwright `request.*` calls (API-style), add `Accept: application/json` header:
```typescript
const resp = await request.post(`${app.baseURL}/containers`, {
  headers: { 'Accept': 'application/json' },
  data: { name: 'Box', parent_id: '' }
});
```

For `page.waitForResponse()` calls, update URL patterns.

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e`
Expected: PASS (may need iterative fixing)

- [ ] **Step 3: Commit**

```bash
git commit -m "test(e2e): update all URLs to unified route scheme"
```

---

### Task 13: Delete old packages + cleanup

**Files:**
- Delete: `internal/api/` (entire directory)
- Delete: `internal/ui/` (entire directory)

- [ ] **Step 1: Remove old packages**

```bash
rm -rf internal/api/ internal/ui/
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: PASS — no remaining imports of `internal/api` or `internal/ui`

- [ ] **Step 3: Run all tests**

Run: `make test`
Expected: PASS

- [ ] **Step 4: Run lint**

Run: `make lint`
Expected: PASS (fix any issues)

- [ ] **Step 5: Commit**

```bash
git commit -m "refactor: remove old api/ and ui/ packages, migration complete"
```

---

### Task 14: Final verification

- [ ] **Step 1: Full test suite**

```bash
make test
make lint
make build-mac
```

- [ ] **Step 2: Manual smoke test**

Start the server and verify:
- Root page loads with container list
- Create/edit/delete containers works
- Create/edit/delete items works
- Tag tree navigation works
- Bulk operations work
- Template designer loads
- Printer management works
- Search works
- Settings/lang switch works

- [ ] **Step 3: E2E tests**

```bash
make test-e2e
```

- [ ] **Step 4: Commit any fixes, then squash/PR**

---

## Risk Notes

1. **Template URL references are pervasive** — ~60 URLs in templates, ~12 in JS. Missing one will cause a broken link. Use `grep -r '/ui/\|/api/' internal/embedded/` to verify completeness after each URL update task.

2. **BindRequest handles string and int fields** — other types (bool, float64) are not handled by form parsing. For now this is fine since only `Quantity int` needs it. JSON body parsing handles all types via `json.Decoder`.

3. **E2E tests need `Accept: application/json`** — Playwright `request.*` calls don't send `HX-Request`, so they'll get full HTML pages by default. Must add `Accept: application/json` header to all API-style requests.

4. **SSE endpoint** — No content negotiation. The handler must remain as-is (raw SSE stream).

5. **`Vary: HX-Request`** — HTMX docs explicitly require this for caching correctness when same URL serves different content based on HX-Request header. HTMLResponder sets this automatically.

6. **Root handler** — Current `GET /` shows containers. After migration, `GET /` must still work. Go 1.22+ `ServeMux` `GET /` is a catch-all for unmatched paths — must check `r.URL.Path == "/"` to avoid catching everything. More specific registered patterns take precedence.

7. **JS tag files use dynamic URL construction** — `tag-autocomplete.js`, `tag-inline.js`, `tag-field.js` build URLs dynamically (e.g., `` `/ui/actions/${objectType}s/${id}/tags` ``). These must be updated to `` `/${objectType}s/${id}/tags` ``. Grep for template literals.

8. **`move-picker.js` and `tag-picker.js` config objects** — these JS files hardcode `/ui/partials/tree` and `/ui/partials/tag-tree` as config properties in JS objects, not just fetch URLs. Update the config values too.
