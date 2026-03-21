# Label Designer Implementation Plan (Phase 1 — MVP)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a visual label template designer using Fabric.js that lets users create, save, and print parameterized label templates for items and containers.

**Architecture:** Fabric.js provides the interactive canvas editor in the browser. Templates are stored in the QLX store as a custom JSON format (QLX format). A converter translates between Fabric objects and QLX JSON. Browser renders PNG via Canvas API, sends to server for printing via a new `PrintImage()` method on PrinterManager.

**Tech Stack:** Go 1.25, Fabric.js v6, vanilla JS, HTMX, HTML templates, qrcode-generator, JsBarcode

**Spec:** `docs/superpowers/specs/2026-03-21-label-designer-design.md`

---

## File Structure

### New files to create

```
internal/store/models.go          — add Template, Asset structs
internal/store/store.go           — add template/asset CRUD methods
internal/embedded/templates/
  templates.html                  — template list page
  template_designer.html          — designer page (canvas + toolbar + properties)
internal/embedded/static/
  fabric.min.js                   — Fabric.js v6 library
  qrcode.min.js                   — qrcode-generator library
  jsbarcode.all.min.js            — JsBarcode library
  label-designer.js               — designer logic (canvas, toolbar, properties)
  qlx-format.js                   — Fabric to QLX JSON converter
  label-params.js                 — parameter substitution engine
  label-dither.js                 — Floyd-Steinberg dithering
  label-print.js                  — print flow (render PNG, POST to server)
```

### Files to modify

```
internal/store/models.go          — add Template, Asset structs
internal/store/store.go           — add template/asset maps + CRUD
internal/ui/server.go             — register new routes, load new templates
internal/ui/handlers.go           — add template + print handlers
internal/print/manager.go         — add PrintImage() method
internal/embedded/embedded.go     — embed new static files
internal/embedded/templates/
  layout.html                     — add Templates nav link
  item.html                       — update print section
  containers.html                 — add batch print section
internal/embedded/static/
  style.css                       — designer styles
  ui-lite.js                      — extend toast for designer
cmd/qlx/main.go                   — create assets directory
```

---

## Task 1: Add Template and Asset models to store

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/store.go`

- [ ] **Step 1: Add Template and Asset structs to models.go**

Add after the `PrinterConfig` struct (line 29):

```go
// Template defines a reusable label layout.
type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Tags      []string  `json:"tags"`
	Target    string    `json:"target"`     // "universal" or "printer:B1"
	WidthMM   float64   `json:"width_mm"`   // universal only
	HeightMM  float64   `json:"height_mm"`  // universal only
	WidthPx   int       `json:"width_px"`   // printer-specific only
	HeightPx  int       `json:"height_px"`  // printer-specific only
	Elements  string    `json:"elements"`   // JSON array of QLX elements
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Asset holds metadata for an uploaded image. Binary data stored on disk.
type Asset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Add templates and assets maps to storeData and Store**

In `store.go`, add `templates` and `assets` fields to `storeData` struct and `Store` struct. Update `NewStore` to initialize maps and accept assets dir path. Update `Save`/load to include new maps.

```go
// In storeData:
type storeData struct {
	Containers map[string]*Container     `json:"containers"`
	Items      map[string]*Item          `json:"items"`
	Printers   map[string]*PrinterConfig `json:"printers"`
	Templates  map[string]*Template      `json:"templates"`
	Assets     map[string]*Asset         `json:"assets"`
}

// In Store struct, add:
	templates  map[string]*Template
	assets     map[string]*Asset
	assetsDir  string
```

Update `NewStore` signature to accept assets dir: `NewStore(path, assetsDir string)`. Initialize new maps. Update `Save` to include templates and assets in serialized data.

**Important:** Also update `NewMemoryStore()` to initialize `templates` and `assets` maps (empty `make(map[...])`). Without this, any code using `NewMemoryStore` will nil-pointer panic.

**Important:** Update all existing call sites of `NewStore` to pass the new `assetsDir` parameter:
- `cmd/qlx/main.go` — pass `filepath.Join(*dataDir, "assets")` (also `os.MkdirAll` the dir)
- `internal/store/store_test.go` — all `NewStore(filepath.Join(...))` calls need a second arg: `filepath.Join(dir, "assets")`
- Note: `manager_test.go` uses `NewMemoryStore()` (no params), which just needs the new maps initialized — no signature change needed there

Note: The existing codebase uses `uuid.New().String()` for ID generation (not a `generateID()` helper). Use the same pattern in all new methods.

- [ ] **Step 3: Verify build and existing tests pass**

Run: `go test ./... -v`
Expected: all existing tests still pass

- [ ] **Step 4: Commit**

```
git add internal/store/models.go internal/store/store.go internal/store/store_test.go internal/print/manager_test.go cmd/qlx/main.go
git commit -m "feat(store): add Template and Asset model structs"
```

---

## Task 2: Implement Template CRUD in store

**Files:**
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write failing tests for template CRUD**

Add to `store_test.go`:

```go
func TestTemplateCRUD(t *testing.T) {
	s := NewMemoryStore()

	tpl := s.CreateTemplate("Test Label", []string{"inventory"}, "universal", 50, 30, 0, 0, `[{"type":"text","x":2,"y":2,"text":"{{name}}"}]`)
	if tpl.ID == "" {
		t.Fatal("CreateTemplate should set ID")
	}
	if tpl.Name != "Test Label" {
		t.Errorf("Name = %q, want %q", tpl.Name, "Test Label")
	}

	got := s.GetTemplate(tpl.ID)
	if got == nil {
		t.Fatal("GetTemplate returned nil")
	}

	all := s.AllTemplates()
	if len(all) != 1 {
		t.Errorf("AllTemplates len = %d, want 1", len(all))
	}

	got.Name = "Updated Label"
	s.SaveTemplate(*got)
	updated := s.GetTemplate(tpl.ID)
	if updated.Name != "Updated Label" {
		t.Errorf("after SaveTemplate Name = %q, want %q", updated.Name, "Updated Label")
	}

	s.DeleteTemplate(tpl.ID)
	if s.GetTemplate(tpl.ID) != nil {
		t.Error("DeleteTemplate: template still exists")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestTemplateCRUD -v`
Expected: FAIL

- [ ] **Step 3: Implement Template CRUD methods**

```go
func (s *Store) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) *Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &Template{
		ID: uuid.New().String(), Name: name, Tags: tags, Target: target,
		WidthMM: widthMM, HeightMM: heightMM, WidthPx: widthPx, HeightPx: heightPx,
		Elements: elements, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.templates[t.ID] = t
	return t
}

func (s *Store) GetTemplate(id string) *Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates[id]
}

func (s *Store) AllTemplates() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Template, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, *t)
	}
	return result
}

func (s *Store) SaveTemplate(t Template) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.UpdatedAt = time.Now()
	s.templates[t.ID] = &t
}

func (s *Store) DeleteTemplate(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.templates, id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestTemplateCRUD -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): implement Template CRUD methods"
```

---

## Task 3: Implement Asset CRUD in store

**Files:**
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write failing tests for asset CRUD**

```go
func TestAssetCRUD(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	s, err := NewStore(filepath.Join(dir, "data.json"), assetsDir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte{0x89, 0x50, 0x4e, 0x47} // PNG header
	asset, err := s.SaveAsset("logo.png", "image/png", data)
	if err != nil {
		t.Fatal(err)
	}
	if asset.ID == "" {
		t.Fatal("SaveAsset should set ID")
	}

	got := s.GetAsset(asset.ID)
	if got == nil {
		t.Fatal("GetAsset returned nil")
	}

	readData, err := s.AssetData(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readData, data) {
		t.Error("AssetData does not match")
	}

	s.DeleteAsset(asset.ID)
	if s.GetAsset(asset.ID) != nil {
		t.Error("DeleteAsset: asset still exists")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestAssetCRUD -v`
Expected: FAIL

- [ ] **Step 3: Implement Asset CRUD methods**

Assets store metadata in JSON, binary data on disk at `{assetsDir}/{id}.bin`.

```go
func (s *Store) SaveAsset(name, mimeType string, data []byte) (*Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &Asset{ID: uuid.New().String(), Name: name, MimeType: mimeType, CreatedAt: time.Now()}
	if err := os.MkdirAll(s.assetsDir, 0755); err != nil {
		return nil, fmt.Errorf("create assets dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.assetsDir, a.ID+".bin"), data, 0644); err != nil {
		return nil, fmt.Errorf("write asset: %w", err)
	}
	s.assets[a.ID] = a
	return a, nil
}

func (s *Store) GetAsset(id string) *Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.assets[id]
}

func (s *Store) AssetData(id string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.assetsDir, id+".bin"))
}

func (s *Store) DeleteAsset(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assets, id)
	os.Remove(filepath.Join(s.assetsDir, id+".bin"))
}

func (s *Store) AllAssets() []Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Asset, 0, len(s.assets))
	for _, a := range s.assets {
		result = append(result, *a)
	}
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestAssetCRUD -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): implement Asset CRUD with file-based storage"
```

---

## Task 4: Add PrintImage method to PrinterManager

**Files:**
- Modify: `internal/print/manager.go`
- Test: `internal/print/manager_test.go`

- [ ] **Step 1: Write failing test**

Use the existing `newManagerWithMock` helper pattern from `manager_test.go`. The test should verify `PrintImage` exists and accepts an `image.Image` argument. It will error on connection (no real printer) which is expected.

```go
func TestPrintImage(t *testing.T) {
	// Use existing test setup pattern from manager_test.go
	m, _ := newManagerWithMock(t)
	p := m.store.AddPrinter("test", "mock", "mock-model", "usb", "/dev/null")

	img := image.NewRGBA(image.Rect(0, 0, 384, 200))
	err := m.PrintImage(p.ID, img)
	// Expect connection error (no real device), not "method not found"
	if err == nil {
		t.Error("expected error (no real printer), got nil")
	}
	if !strings.Contains(err.Error(), "connect") && !strings.Contains(err.Error(), "open") {
		t.Logf("got error: %v (acceptable)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/print/ -run TestPrintImage -v`
Expected: FAIL — `PrintImage` not defined

- [ ] **Step 3: Implement PrintImage**

Add after the existing `Print` method in `manager.go`:

```go
// PrintImage sends a pre-rendered image directly to the printer,
// bypassing label.Render(). Used for browser-side rendered labels.
func (m *PrinterManager) PrintImage(printerID string, img image.Image) error {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	var modelInfo *encoder.ModelInfo
	for _, mi := range enc.Models() {
		if mi.ID == cfg.Model {
			info := mi
			modelInfo = &info
			break
		}
	}
	if modelInfo == nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	m.mu.RLock()
	session, ok := m.sessions[printerID]
	m.mu.RUnlock()

	if !ok || !session.Status().Connected {
		if err := m.ConnectPrinter(printerID); err != nil {
			return fmt.Errorf("connect for print: %w", err)
		}
		m.mu.RLock()
		session = m.sessions[printerID]
		m.mu.RUnlock()
	}

	webutil.LogInfo("printing image on %s (%s/%s)", cfg.Name, cfg.Encoder, cfg.Model)
	opts := encoder.PrintOpts{Density: modelInfo.DensityDefault, AutoCut: true, Quantity: 1}
	if err := session.Print(img, cfg.Model, opts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", cfg.Name)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/print/ -run TestPrintImage -v`
Expected: PASS (error about connection, not missing method)

- [ ] **Step 5: Commit**

```
git add internal/print/manager.go internal/print/manager_test.go
git commit -m "feat(print): add PrintImage method for pre-rendered labels"
```

---

## Task 5: Download and embed JS dependencies

**Files:**
- Create: `internal/embedded/static/fabric.min.js`
- Create: `internal/embedded/static/qrcode.min.js`
- Create: `internal/embedded/static/jsbarcode.all.min.js`

- [ ] **Step 1: Download Fabric.js v6**

```
curl -L "https://cdn.jsdelivr.net/npm/fabric@6/dist/index.min.js" -o internal/embedded/static/fabric.min.js
```

- [ ] **Step 2: Download qrcode-generator**

```
curl -L "https://cdn.jsdelivr.net/npm/qrcode-generator@1.4.4/qrcode.js" -o internal/embedded/static/qrcode.min.js
```

- [ ] **Step 3: Download JsBarcode**

```
curl -L "https://cdn.jsdelivr.net/npm/jsbarcode@3/dist/JsBarcode.all.min.js" -o internal/embedded/static/jsbarcode.all.min.js
```

- [ ] **Step 4: Verify embed covers new files**

The existing `//go:embed static/*` in `internal/embedded/embedded.go` already covers all files in `static/`. No change needed.

- [ ] **Step 5: Verify build**

Run: `go build ./cmd/qlx/`
Expected: compiles

- [ ] **Step 6: Commit**

```
git add internal/embedded/static/fabric.min.js internal/embedded/static/qrcode.min.js internal/embedded/static/jsbarcode.all.min.js
git commit -m "chore: add Fabric.js v6, qrcode-generator, JsBarcode libraries"
```

---

## Task 6: Create template list page

**Files:**
- Create: `internal/embedded/templates/templates.html`
- Modify: `internal/embedded/templates/layout.html`
- Modify: `internal/ui/server.go`
- Modify: `internal/ui/handlers.go`

- [ ] **Step 1: Add Templates nav link to layout.html**

In `layout.html`, add in the `<nav>` after the Printers link:

```html
<a href="/ui/templates" hx-get="/ui/templates" hx-target="#content">Templates</a>
```

- [ ] **Step 2: Create templates.html**

```html
{{ define "templates" }}
<div class="templates-view">
    <h1>Templates</h1>

    {{ if .Tags }}
    <div class="tag-filter">
        {{ range .Tags }}
        <a href="/ui/templates?tag={{ . }}" hx-get="/ui/templates?tag={{ . }}" hx-target="#content" class="tag {{ if eq . $.ActiveTag }}active{{ end }}">{{ . }}</a>
        {{ end }}
        {{ if .ActiveTag }}
        <a href="/ui/templates" hx-get="/ui/templates" hx-target="#content" class="tag">all</a>
        {{ end }}
    </div>
    {{ end }}

    <div class="template-list">
        {{ range .Templates }}
        <div class="card">
            <div class="card-body">
                <h3>{{ .Name }}</h3>
                <span class="badge">{{ .Target }}</span>
                {{ range .Tags }}<span class="tag-badge">{{ . }}</span>{{ end }}
            </div>
            <div class="card-actions">
                <a href="/ui/templates/{{ .ID }}/edit" class="button small">Edit</a>
                <button hx-delete="/ui/actions/templates/{{ .ID }}" hx-target="#content" hx-confirm="Delete this template?" class="danger small">Delete</button>
            </div>
        </div>
        {{ else }}
        <p class="empty">No templates yet.</p>
        {{ end }}
    </div>

    <a href="/ui/templates/new" class="button primary">+ New Template</a>
</div>
{{ end }}
```

- [ ] **Step 3: Add view model and handlers in handlers.go**

**Required new imports in handlers.go:** `"sort"`, `"encoding/json"`, `"strings"`, `"time"`, `"github.com/erxyi/qlx/internal/print/encoder"` (add these as needed across Tasks 6-7).

```go
type TemplateListData struct {
	Templates []store.Template
	Tags      []string
	ActiveTag string
}

func (s *Server) HandleTemplates(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	all := s.store.AllTemplates()

	tagSet := make(map[string]bool)
	var filtered []store.Template
	for _, t := range all {
		for _, tg := range t.Tags {
			tagSet[tg] = true
		}
		if tag == "" || containsTag(t.Tags, tag) {
			filtered = append(filtered, t)
		}
	}
	tags := make([]string, 0, len(tagSet))
	for tg := range tagSet {
		tags = append(tags, tg)
	}
	sort.Strings(tags)

	s.render(w, r, "templates", TemplateListData{Templates: filtered, Tags: tags, ActiveTag: tag})
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func (s *Server) HandleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.store.DeleteTemplate(id)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	s.HandleTemplates(w, r)
}
```

- [ ] **Step 4: Register routes and load template in server.go**

Add `"templates"` to the page templates list in `NewServer`. Register routes:

```go
mux.HandleFunc("GET /ui/templates", s.HandleTemplates)
mux.HandleFunc("DELETE /ui/actions/templates/{id}", s.HandleTemplateDelete)
```

- [ ] **Step 5: Verify build**

Run: `go build ./cmd/qlx/`
Expected: compiles

- [ ] **Step 6: Commit**

```
git add internal/embedded/templates/templates.html internal/embedded/templates/layout.html internal/ui/server.go internal/ui/handlers.go
git commit -m "feat(ui): add template list page with tag filtering"
```

---

## Task 7: Create designer page and handlers

**Files:**
- Create: `internal/embedded/templates/template_designer.html`
- Modify: `internal/ui/server.go`
- Modify: `internal/ui/handlers.go`

- [ ] **Step 1: Create template_designer.html**

```html
{{ define "template-designer" }}
<div class="designer-view" id="designer-app"
     data-template-id="{{ .TemplateID }}"
     data-template-json="{{ .TemplateJSON }}"
     data-printer-models="{{ .PrinterModelsJSON }}"
     data-preview-data="{{ .PreviewDataJSON }}">

    <div class="designer-header">
        <input type="text" id="tpl-name" value="{{ .TemplateName }}" placeholder="Template name" class="designer-name-input">
        <input type="text" id="tpl-tags" value="{{ .TemplateTags }}" placeholder="Tags (comma separated)">
        <select id="tpl-target">
            <option value="universal" {{ if eq .Target "universal" }}selected{{ end }}>Universal (mm)</option>
            {{ range .PrinterModels }}
            <option value="printer:{{ .ID }}" {{ if eq $.Target (printf "printer:%s" .ID) }}selected{{ end }}>{{ .Name }} ({{ .PrintWidthPx }}px)</option>
            {{ end }}
        </select>
        <div class="designer-size">
            <label>W:</label><input type="number" id="tpl-width" value="{{ .Width }}" step="0.5" min="1">
            <label>H:</label><input type="number" id="tpl-height" value="{{ .Height }}" step="0.5" min="1">
            <span id="tpl-unit">mm</span>
        </div>
    </div>

    <div class="designer-body">
        <div class="designer-toolbar" id="toolbar">
            <button data-tool="text" title="Text">T</button>
            <button data-tool="qr" title="QR Code">QR</button>
            <button data-tool="barcode" title="Barcode">BC</button>
            <button data-tool="line" title="Line">--</button>
            <button data-tool="img" title="Image">IMG</button>
            <hr>
            <button data-action="delete" title="Delete selected">DEL</button>
        </div>

        <div class="designer-canvas-wrap">
            <canvas id="label-canvas"></canvas>
        </div>

        <div class="designer-preview-wrap">
            <h4>Preview</h4>
            <canvas id="preview-canvas"></canvas>
        </div>

        <div class="designer-properties" id="properties-panel">
            <h4>Properties</h4>
            <div id="props-content">
                <p class="empty">Select an element</p>
            </div>
        </div>
    </div>

    <div class="designer-footer">
        <a href="/ui/templates" hx-get="/ui/templates" hx-target="#content" class="button">Cancel</a>
        <button id="save-template" class="button primary">Save</button>
    </div>
</div>

<script src="/static/fabric.min.js"></script>
<script src="/static/qrcode.min.js"></script>
<script src="/static/jsbarcode.all.min.js"></script>
<script src="/static/qlx-format.js"></script>
<script src="/static/label-params.js"></script>
<script src="/static/label-dither.js"></script>
<script src="/static/label-print.js"></script>
<script src="/static/label-designer.js"></script>
{{ end }}
```

- [ ] **Step 2: Add designer handlers**

In `handlers.go`:

```go
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

func (s *Server) HandleTemplateNew(w http.ResponseWriter, r *http.Request) {
	models := s.collectPrinterModels()
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(map[string]string{
		"name": "Sample Item", "description": "A sample description",
		"location": "Room > Shelf", "id": "item_123",
		"qr_url": "/ui/items/item_123",
		"date": time.Now().Format("2006-01-02"),
		"time": time.Now().Format("15:04"), "printer": "Default",
	})
	s.render(w, r, "template-designer", DesignerData{
		Target: "universal", Width: 50, Height: 30,
		TemplateJSON: "[]", PrinterModels: models,
		PrinterModelsJSON: string(modelsJSON),
		PreviewDataJSON:   string(previewJSON),
	})
}

func (s *Server) HandleTemplateEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tpl := s.store.GetTemplate(id)
	if tpl == nil {
		http.NotFound(w, r)
		return
	}
	models := s.collectPrinterModels()
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(map[string]string{
		"name": "Sample Item", "description": "A sample description",
		"location": "Room > Shelf", "id": "item_123",
		"qr_url": "/ui/items/item_123",
		"date": time.Now().Format("2006-01-02"),
		"time": time.Now().Format("15:04"), "printer": "Default",
	})
	width, height := tpl.WidthMM, tpl.HeightMM
	if strings.HasPrefix(tpl.Target, "printer:") {
		width, height = float64(tpl.WidthPx), float64(tpl.HeightPx)
	}
	s.render(w, r, "template-designer", DesignerData{
		TemplateID: tpl.ID, TemplateName: tpl.Name,
		TemplateTags: strings.Join(tpl.Tags, ", "),
		Target: tpl.Target, Width: width, Height: height,
		TemplateJSON: tpl.Elements, PrinterModels: models,
		PrinterModelsJSON: string(modelsJSON),
		PreviewDataJSON:   string(previewJSON),
	})
}

func (s *Server) HandleTemplateSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Tags     string  `json:"tags"`
		Target   string  `json:"target"`
		Width    float64 `json:"width"`
		Height   float64 `json:"height"`
		Elements string  `json:"elements"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tags := splitTags(req.Tags)

	if req.ID != "" {
		tpl := s.store.GetTemplate(req.ID)
		if tpl == nil {
			http.NotFound(w, r)
			return
		}
		tpl.Name = req.Name
		tpl.Tags = tags
		tpl.Target = req.Target
		if req.Target == "universal" {
			tpl.WidthMM, tpl.HeightMM = req.Width, req.Height
		} else {
			tpl.WidthPx, tpl.HeightPx = int(req.Width), int(req.Height)
		}
		tpl.Elements = req.Elements
		s.store.SaveTemplate(*tpl)
	} else {
		widthMM, heightMM := req.Width, req.Height
		var widthPx, heightPx int
		if req.Target != "universal" {
			widthPx, heightPx = int(req.Width), int(req.Height)
			widthMM, heightMM = 0, 0
		}
		s.store.CreateTemplate(req.Name, tags, req.Target, widthMM, heightMM, widthPx, heightPx, req.Elements)
	}

	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func (s *Server) collectPrinterModels() []encoder.ModelInfo {
	var models []encoder.ModelInfo
	for _, enc := range s.printerManager.AvailableEncoders() {
		models = append(models, enc.Models()...)
	}
	return models
}
```

- [ ] **Step 3: Register routes in server.go**

```go
mux.HandleFunc("GET /ui/templates/new", s.HandleTemplateNew)
mux.HandleFunc("GET /ui/templates/{id}/edit", s.HandleTemplateEdit)
mux.HandleFunc("POST /ui/actions/templates", s.HandleTemplateSave)
mux.HandleFunc("PUT /ui/actions/templates/{id}", s.HandleTemplateSave)
```

In `NewServer`, add to the `templateFiles` map: `"template-designer": "templates/template_designer.html"`. The key must match the `{{ define "template-designer" }}` in the HTML file.

- [ ] **Step 4: Verify build**

Run: `go build ./cmd/qlx/`
Expected: compiles

- [ ] **Step 5: Commit**

```
git add internal/embedded/templates/template_designer.html internal/ui/server.go internal/ui/handlers.go
git commit -m "feat(ui): add designer page with new/edit/save handlers"
```

---

## Task 8: Implement QLX format converter (JS)

**Files:**
- Create: `internal/embedded/static/qlx-format.js`

- [ ] **Step 1: Create qlx-format.js**

Converts between Fabric.js objects and QLX JSON format. Handles all 5 element types: text, qr, barcode, line, img. Includes QR and barcode rendering helpers using qrcode-generator and JsBarcode.

Key functions:
- `QlxFormat.canvasToQlx(canvas)` — serialize Fabric canvas to QLX elements array
- `QlxFormat.qlxToCanvas(canvas, elements, params)` — load QLX elements onto Fabric canvas with optional parameter substitution
- `QlxFormat.substituteParams(text, params)` — replace `{{key}}` with values

Each Fabric object gets custom properties: `qlxType`, `qlxTemplate` (for text), `qlxContent` (for qr/barcode), `qlxSrc`/`qlxFit` (for img).

Line coordinate conversion: QLX uses absolute `x1,y1,x2,y2`. Fabric uses `left,top` + relative offsets. Converter translates between the two.

- [ ] **Step 2: Commit**

```
git add internal/embedded/static/qlx-format.js
git commit -m "feat(js): implement QLX format converter (Fabric <-> QLX JSON)"
```

---

## Task 9: Implement dithering module (JS)

**Files:**
- Create: `internal/embedded/static/label-dither.js`

- [ ] **Step 1: Create label-dither.js**

Floyd-Steinberg dithering for monochrome output. Takes a source canvas, returns a new canvas with black/white pixels only.

```javascript
window.LabelDither = (function() {
  function dither(sourceCanvas) {
    var w = sourceCanvas.width, h = sourceCanvas.height;
    var src = sourceCanvas.getContext('2d').getImageData(0, 0, w, h);
    var data = new Float32Array(w * h);

    // Convert to grayscale
    for (var i = 0; i < w * h; i++) {
      data[i] = 0.299 * src.data[i*4] + 0.587 * src.data[i*4+1] + 0.114 * src.data[i*4+2];
    }

    // Floyd-Steinberg error diffusion
    for (var y = 0; y < h; y++) {
      for (var x = 0; x < w; x++) {
        var idx = y * w + x;
        var oldVal = data[idx];
        var newVal = oldVal < 128 ? 0 : 255;
        data[idx] = newVal;
        var err = oldVal - newVal;
        if (x + 1 < w)              data[idx + 1]     += err * 7/16;
        if (y + 1 < h && x > 0)     data[idx + w - 1] += err * 3/16;
        if (y + 1 < h)              data[idx + w]     += err * 5/16;
        if (y + 1 < h && x + 1 < w) data[idx + w + 1] += err * 1/16;
      }
    }

    // Output
    var out = document.createElement('canvas');
    out.width = w; out.height = h;
    var ctx = out.getContext('2d');
    var outData = ctx.createImageData(w, h);
    for (var i = 0; i < w * h; i++) {
      var v = data[i] < 128 ? 0 : 255;
      outData.data[i*4] = outData.data[i*4+1] = outData.data[i*4+2] = v;
      outData.data[i*4+3] = 255;
    }
    ctx.putImageData(outData, 0, 0);
    return out;
  }
  return { dither: dither };
})();
```

- [ ] **Step 2: Commit**

```
git add internal/embedded/static/label-dither.js
git commit -m "feat(js): add Floyd-Steinberg dithering for monochrome labels"
```

---

## Task 10: Implement parameter substitution and print flow (JS)

**Files:**
- Create: `internal/embedded/static/label-params.js`
- Create: `internal/embedded/static/label-print.js`

- [ ] **Step 1: Create label-params.js**

Provides `LabelParams.buildContext(entity, printerName)` to build parameter map and `LabelParams.substitute(text, params)` for `{{key}}` replacement.

- [ ] **Step 2: Create label-print.js**

Provides `LabelPrint.print(canvas, printerId, multiplier)` which:
1. Exports Fabric canvas to PNG via `toDataURL()`
2. Applies Floyd-Steinberg dithering
3. POSTs base64 PNG to `/ui/actions/print-image` (the new generic endpoint from Task 14)
4. Returns a Promise

**Note on print endpoints:** The existing `POST /ui/actions/items/{id}/print` (using `HandleItemPrint` with `label.Render()`) remains as legacy fallback. The new designer flow uses the generic `POST /ui/actions/print-image` endpoint which accepts pre-rendered PNGs. Both `item.html` and `containers.html` will call the new endpoint when using designer templates.

- [ ] **Step 3: Verify all JS files have no syntax errors**

Run: `go build ./cmd/qlx/` (ensures all JS files embed correctly)

- [ ] **Step 4: Commit**

```
git add internal/embedded/static/label-params.js internal/embedded/static/label-print.js
git commit -m "feat(js): add parameter substitution and print flow modules"
```

---

## Task 11: Implement main designer JS

**Files:**
- Create: `internal/embedded/static/label-designer.js`

- [ ] **Step 1: Create label-designer.js**

Main designer controller that:
- Reads template data from `data-*` attributes on `#designer-app`
- Initializes Fabric.Canvas with correct dimensions
- Loads existing elements via `QlxFormat.qlxToCanvas()`
- Handles toolbar clicks (add text/qr/barcode/line/img)
- Shows properties panel on object selection
- Updates live preview on changes using `QlxFormat.canvasToQlx()` + `QlxFormat.qlxToCanvas()` with real data
- Saves template via POST/PUT to `/ui/actions/templates`
- Handles asset upload for img elements
- Manages target/size changes (universal mm vs printer px)

- [ ] **Step 2: Commit**

```
git add internal/embedded/static/label-designer.js
git commit -m "feat(js): implement main label designer with Fabric.js canvas"
```

---

## Task 12: Add designer CSS styles

**Files:**
- Modify: `internal/embedded/static/style.css`

- [ ] **Step 1: Append designer styles to style.css**

Add CSS for: `.designer-view`, `.designer-header`, `.designer-body`, `.designer-toolbar`, `.designer-canvas-wrap`, `.designer-preview-wrap`, `.designer-properties`, `.designer-footer`, `.prop-group`, `.tag-filter`, `.tag`, `.tag-badge`, `.badge`, `.card-actions`, responsive breakpoints.

Follow existing dark theme variables (`--bg-alt`, `--card-bg`, `--border`, `--text`, `--accent`, `--success`).

- [ ] **Step 2: Commit**

```
git add internal/embedded/static/style.css
git commit -m "feat(css): add label designer and template list styles"
```

---

## Task 13: Update item.html print section

**Files:**
- Modify: `internal/embedded/templates/item.html`
- Modify: `internal/ui/handlers.go`

- [ ] **Step 1: Update ItemDetailData to include templates**

Add `Templates []store.Template` to `ItemDetailData`. Update `HandleItem` to pass `s.store.AllTemplates()`.

- [ ] **Step 2: Replace hardcoded template dropdown in item.html**

Replace the existing `<select id="print-template">` with dynamic list from `.Templates`, keeping legacy options as fallback.

- [ ] **Step 3: Commit**

```
git add internal/embedded/templates/item.html internal/ui/handlers.go
git commit -m "feat(ui): update item print with template selection"
```

---

## Task 14: Add print-image and asset endpoints

**Files:**
- Modify: `internal/ui/handlers.go`
- Modify: `internal/ui/server.go`

- [ ] **Step 1: Add HandlePrintImage handler**

Accepts `{printer_id, png}` JSON where `png` is `data:image/png;base64,...`. Decodes base64, parses PNG, calls `PrinterManager.PrintImage()`.

- [ ] **Step 2: Add HandleAssetUpload and HandleAssetServe handlers**

Upload: accepts multipart form with `file` field, saves via `store.SaveAsset()`, returns `{id, name}`.
Serve: reads asset by ID, serves with correct Content-Type.

- [ ] **Step 3: Register routes**

```go
mux.HandleFunc("POST /ui/actions/print-image", s.HandlePrintImage)
mux.HandleFunc("POST /ui/actions/assets", s.HandleAssetUpload)
mux.HandleFunc("GET /ui/actions/assets/{id}", s.HandleAssetServe)
```

- [ ] **Step 4: Commit**

```
git add internal/ui/handlers.go internal/ui/server.go
git commit -m "feat(ui): add print-image and asset upload/serve endpoints"
```

---

## Task 15: Add container print section

**Files:**
- Modify: `internal/embedded/templates/containers.html`
- Modify: `internal/ui/handlers.go`

- [ ] **Step 1: Update ContainerListData with printers and templates**

Add `Printers []store.PrinterConfig` and `Templates []store.Template` to `ContainerListData`. Update `HandleContainer` and `HandleRoot` to pass them.

- [ ] **Step 2: Add print section to containers.html**

After items list, add print section with: printer dropdown, template dropdown, "Print Container Label" button, and batch print section with "Include sub-containers" checkbox and "Print All (N)" button. Print buttons use JS to render via Fabric canvas → dither → POST to `/ui/actions/print-image` (same flow as item print).

For batch print, the JS iterates over items client-side: for each item, renders the template with that item's data, dithers, and POSTs sequentially with a small delay between prints.

- [ ] **Step 3: Add container print and batch print endpoint stubs**

Container and batch print endpoints are needed for the UI buttons. For Phase 1 (browser rendering), these are thin wrappers — the real rendering happens client-side. The endpoints serve the container/item data needed for JS rendering:

```go
// GET endpoint to fetch items for batch printing
func (s *Server) HandleContainerItemsJSON(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	recursive := r.URL.Query().Get("recursive") == "true"
	items := s.store.ContainerItems(id)
	if recursive {
		items = s.collectItemsRecursive(id)
	}
	// Build label data for each item
	var result []map[string]string
	for _, item := range items {
		path := s.store.ContainerPath(item.ContainerID)
		// Build location string inline (same pattern as HandleItemPrint in handlers.go)
		var parts []string
		for _, c := range path {
			parts = append(parts, c.Name)
		}
		loc := strings.Join(parts, " → ")
		result = append(result, map[string]string{
			"name": item.Name, "description": item.Description,
			"location": loc, "id": item.ID,
			"qr_url": "/ui/items/" + item.ID,
		})
	}
	webutil.JSON(w, http.StatusOK, result)
}

func (s *Server) collectItemsRecursive(containerID string) []store.Item {
	var all []store.Item
	all = append(all, s.store.ContainerItems(containerID)...)
	for _, child := range s.store.ContainerChildren(containerID) {
		all = append(all, s.collectItemsRecursive(child.ID)...)
	}
	return all
}
```

Register:
```go
mux.HandleFunc("GET /ui/actions/containers/{id}/items-json", s.HandleContainerItemsJSON)
```

- [ ] **Step 4: Commit**

```
git add internal/embedded/templates/containers.html internal/ui/handlers.go internal/ui/server.go
git commit -m "feat(ui): add container label and batch print sections"
```

---

## Task 16: (merged into Task 1)

The `main.go` update for assets directory was merged into Task 1 to avoid breaking the build between tasks.

---

## Task 17: End-to-end verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: all pass

- [ ] **Step 2: Build and smoke test**

Run: `go build -o qlx ./cmd/qlx/ && ./qlx -port 9999 -data /tmp/qlx-test`

Manually verify:
1. Navigate to `http://localhost:9999/ui/templates` — see empty list
2. Click "+ New Template" — designer loads with Fabric canvas
3. Add text element — appears on canvas, properties panel updates
4. Type `{{name}}` in text — preview shows "Sample Item"
5. Save template — redirects to list, template appears
6. Edit template — designer loads with saved elements
7. Navigate to item — template appears in print dropdown

- [ ] **Step 3: Final commit**

```
git add -A
git commit -m "feat: label designer MVP complete (Phase 1)"
```

---

## Summary

| Task | Description |
|------|-------------|
| 1 | Template + Asset models |
| 2 | Template CRUD in store |
| 3 | Asset CRUD in store |
| 4 | PrintImage on PrinterManager |
| 5 | Download JS dependencies |
| 6 | Template list page + handlers |
| 7 | Designer page (HTML + handlers) |
| 8 | QLX format converter (JS) |
| 9 | Dithering module (JS) |
| 10 | Parameter substitution + print flow (JS) |
| 11 | Main designer (JS) |
| 12 | Designer CSS |
| 13 | Update item print section |
| 14 | Print-image and asset endpoints |
| 15 | Container print section |
| 16 | Main.go assets dir |
| 17 | End-to-end verification |

**Deferred to Phase 3:** Template duplication (`POST /ui/actions/templates/{id}/duplicate`). Listed in spec's Phase 3 polish section.
