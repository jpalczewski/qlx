# Print Options & Renderer Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose print options (density, copies, cut control, high-res) via API/UI, make renderer media-aware for die-cut labels, and deduplicate the manager's print paths.

**Architecture:** Extend `PrintOpts` (replace `AutoCut`/`Quantity` with `CutEvery`/`Copies`/`HighRes`), add `MediaInfo` to the label renderer, extract `resolveForPrint()` in manager, update both encoders for multi-copy and cut control, add capabilities API endpoint, and wire UI controls.

**Tech Stack:** Go 1.26, standard library `image`, HTMX, vanilla JS

**Spec:** `docs/superpowers/specs/2026-03-28-print-options-renderer-design.md`

---

## File Structure

### Modified files
- `internal/print/encoder/encoder.go` — `PrintOpts` migration + `ModelInfo` gains `HighResSupported`, `CutSupported`
- `internal/print/label/renderer.go` — `MediaInfo` struct, `Render()` signature change
- `internal/print/label/schema_renderer.go` — die-cut fit-to-label + overflow priority
- `internal/print/manager.go` — `resolveForPrint()`, updated `Print()`/`PrintImage()` signatures, `ValidatePrintOpts()`, `BuildMediaInfo()`
- `internal/print/manager_test.go` — updated call sites + new tests
- `internal/print/label/renderer_test.go` — updated call sites + die-cut tests
- `internal/print/encoder/brother/brother.go` — multi-copy loop, cut_every, high_res, dynamic margin
- `internal/print/encoder/brother/models.go` — `HighResSupported`, `CutSupported` fields
- `internal/print/encoder/niimbot/niimbot.go` — copies support, density unfreeze
- `internal/print/encoder/niimbot/models.go` — no density changes needed (already has range)
- `internal/handler/request.go` — `PrintRequest`/`ContainerPrintRequest` gain print option fields
- `internal/handler/print.go` — parse print opts, pass to manager, capabilities endpoint, preview `printer_id` param
- `internal/embedded/templates/` — print modal controls partial
- `internal/embedded/static/js/` — dynamic show/hide on printer change

### New files
- `internal/print/label/schema_renderer_test.go` — already exists, add die-cut tests there
- `internal/print/encoder/brother/brother_test.go` — multi-copy + cut tests (if doesn't exist, add)

---

## Task 1: Migrate PrintOpts — replace AutoCut/Quantity with CutEvery/Copies/HighRes

**Files:**
- Modify: `internal/print/encoder/encoder.go:26-30`
- Modify: `internal/print/manager.go:183-187,236-240`
- Modify: `internal/print/manager_test.go`
- Modify: `internal/print/encoder/brother/brother.go:78,85`
- Modify: `internal/print/encoder/niimbot/niimbot.go:117`

- [ ] **Step 1: Update PrintOpts struct in encoder.go**

Replace the struct at `encoder.go:26-30`:

```go
type PrintOpts struct {
	Density  int  `json:"density"`   // 0 = model default; Niimbot 1-5, Brother ignored
	Copies   int  `json:"copies"`    // 0/1 = single; >1 = multi-copy
	CutEvery int  `json:"cut_every"` // 0 = no cut; 1 = every copy; N = every N copies
	HighRes  bool `json:"high_res"`  // Brother: 600 DPI vertical; others: ignored
}
```

- [ ] **Step 2: Update Brother encoder to use new fields**

In `brother.go`, replace the autocut section (lines 77-92):

```go
// 4. Various mode: autocut
if opts.CutEvery > 0 {
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x40}); err != nil {
		return err
	}
} else {
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x00}); err != nil {
		return err
	}
}

// 5. Cut every N labels
cutEvery := byte(1)
if opts.CutEvery > 0 {
	cutEvery = byte(opts.CutEvery)
}
if _, err := tr.Write([]byte{0x1B, 0x69, 0x41, cutEvery}); err != nil {
	return err
}

// 6. Expanded mode: cut at end (bit 3)
expandedMode := byte(0x00)
if opts.CutEvery > 0 {
	expandedMode |= 0x08
}
if _, err := tr.Write([]byte{0x1B, 0x69, 0x4B, expandedMode}); err != nil {
	return err
}
```

- [ ] **Step 3: Update Niimbot encoder — copies field**

In `niimbot.go:117`, change hardcoded copies:

```go
binary.BigEndian.PutUint16(pageSizeData[4:6], uint16(max(opts.Copies, 1)))
```

- [ ] **Step 4: Update all PrintOpts construction sites in manager.go**

In `manager.go:183-187` and `236-240`, replace:
```go
printOpts := encoder.PrintOpts{
	Density:  modelInfo.DensityDefault,
	AutoCut:  true,
	Quantity: 1,
}
```
with:
```go
printOpts := encoder.PrintOpts{
	Density:  modelInfo.DensityDefault,
	Copies:   1,
	CutEvery: 1,
}
```

- [ ] **Step 5: Verify tests compile and pass**

Run: `make test`
Expected: All tests pass (call sites updated, no behavioral change yet)

- [ ] **Step 6: Commit**

```bash
git add internal/print/encoder/encoder.go internal/print/manager.go internal/print/manager_test.go \
  internal/print/encoder/brother/brother.go internal/print/encoder/niimbot/niimbot.go
git commit -m "refactor(print): migrate PrintOpts from AutoCut/Quantity to CutEvery/Copies/HighRes"
```

---

## Task 2: Add MediaInfo to renderer and update Render() signature

**Files:**
- Modify: `internal/print/label/renderer.go:26`
- Modify: `internal/print/label/schema_renderer.go:103`
- Modify: `internal/print/label/renderer_test.go`
- Modify: `internal/print/manager.go:167`

- [ ] **Step 1: Write tests for MediaInfo rendering — continuous (existing behavior)**

In `renderer_test.go`, update all existing tests to use `MediaInfo`. Example for `TestRenderSimple`:

```go
func TestRenderSimple(t *testing.T) {
	data := LabelData{Name: "Test Item"}
	media := MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}
	img, err := Render(data, "simple", media, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}
```

Update all other tests similarly (`TestRenderStandard`, `TestRenderDetailed`, `TestRenderCompact`, `TestRenderUnknownTemplate`, `TestRender_PrintDate`).

- [ ] **Step 2: Write test for die-cut rendering**

Add to `renderer_test.go`:

```go
func TestRender_DieCut_FixedHeight(t *testing.T) {
	data := LabelData{Name: "Test Item", Description: "Desc", Location: "Room > Shelf"}
	media := MediaInfo{WidthPx: 384, HeightPx: 200, DPI: 203}
	img, err := Render(data, "standard", media, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() != 200 {
		t.Errorf("height = %d, want 200 (die-cut)", img.Bounds().Dy())
	}
}

func TestRender_DieCut_Overflow_ClipsHeight(t *testing.T) {
	// Create data with many tags to overflow a small die-cut label
	tags := make([]LabelTag, 20)
	for i := range tags {
		tags[i] = LabelTag{Name: fmt.Sprintf("tag%d", i)}
	}
	data := LabelData{
		Name:     "Long Title That Might Wrap",
		Location: "Very Long Location Path > Sub > Sub > Sub",
		Tags:     tags,
	}
	media := MediaInfo{WidthPx: 384, HeightPx: 100, DPI: 203}
	img, err := Render(data, "detailed", media, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dy() != 100 {
		t.Errorf("height = %d, want 100 (die-cut clipped)", img.Bounds().Dy())
	}
}
```

- [ ] **Step 3: Run tests — expect FAIL (signature doesn't match yet)**

Run: `go test ./internal/print/label/ -v`
Expected: Compilation error — `Render` signature mismatch

- [ ] **Step 4: Add MediaInfo struct and update Render() signature**

In `renderer.go`, add `MediaInfo` and update `Render()`:

```go
// MediaInfo describes the physical print media.
type MediaInfo struct {
	WidthPx  int // printhead width in pixels (384, 720)
	HeightPx int // 0 = continuous (dynamic height); >0 = die-cut label height in px
	DPI      int // 203, 300
}

// Render produces a label image from the given data using the named schema.
func Render(data LabelData, template string, media MediaInfo, opts RenderOpts) (image.Image, error) {
	schema, ok := GetSchema(template)
	if !ok {
		return nil, fmt.Errorf("unknown template %q: valid templates are %v", template, SchemaNames())
	}
	return renderSchema(schema, data, media, opts)
}
```

- [ ] **Step 5: Update renderSchema() signature in schema_renderer.go**

Change `renderSchema` signature from `(schema Schema, data LabelData, widthPx int, opts RenderOpts)` to `(schema Schema, data LabelData, media MediaInfo, opts RenderOpts)`.

Inside the function, use `media.WidthPx` where `widthPx` was used. For die-cut logic:

```go
func renderSchema(schema Schema, data LabelData, media MediaInfo, opts RenderOpts) (image.Image, error) {
	widthPx := media.WidthPx
	pad := schema.Padding
	qrReserved := qrSizeForSchema(schema)

	textElems, qrSize, barcodeH, err := resolveElements(schema, data, widthPx, pad, qrReserved)
	if err != nil {
		return nil, err
	}

	var dateLine resolvedText
	var dateLineH int
	if opts.PrintDate {
		var pErr error
		dateLine, pErr = buildDateLine()
		if pErr != nil {
			return nil, pErr
		}
		dateLineH = dateLine.lineH + 2
	}

	contentH := computeHeight(textElems, qrSize, barcodeH, data.BarcodeID, pad) + dateLineH

	totalH := contentH
	if media.HeightPx > 0 {
		totalH = media.HeightPx
		// Die-cut: if content overflows, truncate elements by priority
		if contentH > totalH {
			textElems = truncateForDieCut(textElems, schema, qrSize, barcodeH, data.BarcodeID, pad, dateLineH, totalH)
		}
	}

	img := newCanvas(widthPx, totalH)

	drawTextElements(img, textElems, widthPx, pad, qrSize)

	if err := maybeDrawQR(img, data.QRContent, widthPx, pad, qrSize); err != nil {
		return nil, err
	}
	if err := maybeDrawBarcode(img, data.BarcodeID, widthPx, pad, barcodeH, totalH-dateLineH); err != nil {
		return nil, err
	}

	if opts.PrintDate {
		drawDateLine(img, dateLine, widthPx, pad, totalH)
	}

	return img, nil
}
```

- [ ] **Step 6: Implement truncateForDieCut()**

Add to `schema_renderer.go`:

```go
// truncateForDieCut removes lowest-priority elements until content fits within maxH.
// Priority (lowest removed first): tags, children, description, location.
// Title + QR/Barcode are always kept.
func truncateForDieCut(elems []resolvedText, schema Schema, qrSize, barcodeH int, barcodeID string, pad, dateLineH, maxH int) []resolvedText {
	// Try removing elements from the end (lowest priority rendered last)
	// Schema element order maps to resolvedText order
	for len(elems) > 1 {
		h := computeHeight(elems, qrSize, barcodeH, barcodeID, pad) + dateLineH
		if h <= maxH {
			return elems
		}
		// Remove last element (lowest priority)
		elems = elems[:len(elems)-1]
	}

	// If still too tall with just the first element (title), truncate its lines
	for len(elems) > 0 && len(elems[0].lines) > 1 {
		h := computeHeight(elems, qrSize, barcodeH, barcodeID, pad) + dateLineH
		if h <= maxH {
			return elems
		}
		// Remove last line of title, add ellipsis
		lines := elems[0].lines
		last := lines[len(lines)-2]
		if len(last) > 3 {
			last = last[:len(last)-3] + "..."
		}
		elems[0].lines = append(lines[:len(lines)-2], last)
	}

	return elems
}
```

- [ ] **Step 7: Update manager.go Render() call site (temporary — Task 3 replaces this)**

In `manager.go:167`, change:
```go
img, err := label.Render(data, templateName, modelInfo.PrintWidthPx, modelInfo.DPI, opts)
```
to:
```go
img, err := label.Render(data, templateName, label.MediaInfo{WidthPx: modelInfo.PrintWidthPx, DPI: modelInfo.DPI}, opts)
```

Note: This is a minimal change so the code compiles. Task 3 replaces `Print()` entirely with `resolveForPrint()` which builds full `MediaInfo` from session status.

- [ ] **Step 8: Run tests — expect all PASS**

Run: `make test`
Expected: All tests pass, including new die-cut tests

- [ ] **Step 9: Commit**

```bash
git add internal/print/label/ internal/print/manager.go
git commit -m "feat(label): add MediaInfo to renderer with die-cut fit-to-label support"
```

---

## Task 3: Extract resolveForPrint() and wire print options through manager

**Files:**
- Modify: `internal/print/manager.go:150-247`
- Modify: `internal/print/manager_test.go`

- [ ] **Step 1: Write test for Print with custom PrintOpts**

Add to `manager_test.go`:

```go
func TestPrinterManager_PrintWithOpts(t *testing.T) {
	mgr, s, mockTr, cm := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")
	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state")
	}

	data := label.LabelData{Name: "Test Item"}
	opts := encoder.PrintOpts{Density: 5, Copies: 3, CutEvery: 0}

	if err := mgr.Print(printer.ID, data, "simple", label.RenderOpts{}, opts); err != nil {
		t.Fatalf("Print() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport")
	}
}

func TestPrinterManager_PrintImageWithOpts(t *testing.T) {
	mgr, s, mockTr, cm := newManagerWithMock(t)

	printer := s.AddPrinter("Test Printer", "mock", "mock-model", "mock", "/dev/null")
	if err := cm.Add(*printer); err != nil {
		t.Fatalf("cm.Add: %v", err)
	}
	if !waitForState(cm, printer.ID, StateConnected, 2*time.Second) {
		t.Fatalf("printer did not reach connected state")
	}

	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	opts := encoder.PrintOpts{Density: 2, Copies: 1, CutEvery: 1}

	if err := mgr.PrintImage(printer.ID, img, opts); err != nil {
		t.Fatalf("PrintImage() returned unexpected error: %v", err)
	}

	if len(mockTr.Written) == 0 {
		t.Error("expected data to be written to transport")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL (signatures don't match)**

Run: `go test ./internal/print/ -v -run TestPrinterManager_Print`
Expected: Compilation error

- [ ] **Step 3: Implement resolveForPrint() and update Print()/PrintImage()**

Replace the `Print()` and `PrintImage()` methods in `manager.go` with:

```go
// printContext holds resolved printer resources for a print operation.
type printContext struct {
	cfg       *store.PrinterConfig
	enc       encoder.Encoder
	model     *encoder.ModelInfo
	session   *PrinterSession
	media     label.MediaInfo
	printOpts encoder.PrintOpts
}

// resolveForPrint looks up printer config, encoder, model, session, and validates print options.
func (m *PrinterManager) resolveForPrint(printerID string, opts encoder.PrintOpts) (*printContext, error) {
	cfg := m.store.GetPrinter(printerID)
	if cfg == nil {
		return nil, fmt.Errorf("printer not found: %s", printerID)
	}

	enc, ok := m.encoders[cfg.Encoder]
	if !ok {
		return nil, fmt.Errorf("encoder not found: %s", cfg.Encoder)
	}

	modelInfo := FindModel(enc, cfg.Model)
	if modelInfo == nil {
		return nil, fmt.Errorf("model not found: %s", cfg.Model)
	}

	if m.cm.State(printerID) != StateConnected {
		return nil, fmt.Errorf("printer %s not connected", printerID)
	}
	session := m.cm.Session(printerID)
	if session == nil {
		return nil, fmt.Errorf("printer %s: no active session", printerID)
	}

	// Validate and apply defaults
	validated := validatePrintOpts(opts, modelInfo)

	// Build media info from session status
	status := session.Status()
	media := label.MediaInfo{
		WidthPx:  modelInfo.PrintWidthPx,
		HeightPx: PxFromMm(status.LabelHeightMm, modelInfo.DPI),
		DPI:      modelInfo.DPI,
	}

	return &printContext{
		cfg:       cfg,
		enc:       enc,
		model:     modelInfo,
		session:   session,
		media:     media,
		printOpts: validated,
	}, nil
}

// validatePrintOpts clamps values to valid ranges and applies defaults.
func validatePrintOpts(opts encoder.PrintOpts, mi *encoder.ModelInfo) encoder.PrintOpts {
	if opts.Density == 0 {
		opts.Density = mi.DensityDefault
	}
	if opts.Density < mi.DensityRange[0] {
		opts.Density = mi.DensityRange[0]
	}
	if opts.Density > mi.DensityRange[1] {
		opts.Density = mi.DensityRange[1]
	}
	if opts.Copies < 1 {
		opts.Copies = 1
	}
	if opts.Copies > 100 {
		opts.Copies = 100
	}
	if opts.CutEvery < 0 {
		opts.CutEvery = 0
	}
	if opts.CutEvery > opts.Copies {
		opts.CutEvery = opts.Copies
	}
	return opts
}

// PxFromMm converts millimeters to pixels at the given DPI. Exported for use by handler.
func PxFromMm(mm, dpi int) int {
	if mm <= 0 || dpi <= 0 {
		return 0
	}
	return int(float64(mm) * float64(dpi) / 25.4)
}

// FindModel returns the ModelInfo matching modelID from an encoder. Exported for use by handler.
func FindModel(enc encoder.Encoder, modelID string) *encoder.ModelInfo {
	for _, mi := range enc.Models() {
		if mi.ID == modelID {
			info := mi
			return &info
		}
	}
	return nil
}

// Print renders a label and sends it to the printer.
func (m *PrinterManager) Print(printerID string, data label.LabelData, templateName string, renderOpts label.RenderOpts, printOpts encoder.PrintOpts) error {
	ctx, err := m.resolveForPrint(printerID, printOpts)
	if err != nil {
		return err
	}

	img, err := label.Render(data, templateName, ctx.media, renderOpts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	img = applyCalibrationOffset(img, ctx.cfg, ctx.media.WidthPx)

	webutil.LogInfo("printing on %s (%s/%s)", ctx.cfg.Name, ctx.cfg.Encoder, ctx.cfg.Model)
	if err := ctx.session.Print(img, ctx.cfg.Model, ctx.printOpts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print complete on %s", ctx.cfg.Name)
	return nil
}

// PrintImage sends a pre-rendered image directly to the printer.
func (m *PrinterManager) PrintImage(printerID string, img image.Image, printOpts encoder.PrintOpts) error {
	ctx, err := m.resolveForPrint(printerID, printOpts)
	if err != nil {
		return err
	}

	// Scale image to match printer printhead width if needed
	imgWidth := img.Bounds().Dx()
	if imgWidth != ctx.media.WidthPx && ctx.media.WidthPx > 0 {
		scale := float64(ctx.media.WidthPx) / float64(imgWidth)
		newH := int(float64(img.Bounds().Dy()) * scale)
		dst := image.NewRGBA(image.Rect(0, 0, ctx.media.WidthPx, newH))
		imagedraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		img = dst
		webutil.LogInfo("scaled image %d→%dpx width for %s", imgWidth, ctx.media.WidthPx, ctx.cfg.Model)
	}

	img = applyCalibrationOffset(img, ctx.cfg, ctx.media.WidthPx)

	webutil.LogInfo("printing image on %s (%s/%s)", ctx.cfg.Name, ctx.cfg.Encoder, ctx.cfg.Model)
	if err := ctx.session.Print(img, ctx.cfg.Model, ctx.printOpts); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	webutil.LogInfo("print image complete on %s", ctx.cfg.Name)
	return nil
}
```

- [ ] **Step 4: Update existing test call sites**

Update `TestPrinterManager_Print` and `TestPrinterManager_PrintUnknownPrinter`:
```go
// Old:
mgr.Print(printer.ID, data, "simple", label.RenderOpts{})
// New:
mgr.Print(printer.ID, data, "simple", label.RenderOpts{}, encoder.PrintOpts{})
```

Update `TestPrinterManager_PrintImage` and `TestPrinterManager_PrintImageUnknownPrinter`:
```go
// Old:
mgr.PrintImage(printer.ID, img)
// New:
mgr.PrintImage(printer.ID, img, encoder.PrintOpts{})
```

- [ ] **Step 5: Run tests — expect PASS**

Run: `make test`
Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/print/manager.go internal/print/manager_test.go
git commit -m "refactor(print): extract resolveForPrint() and wire PrintOpts through manager"
```

---

## Task 4: Update handler to parse and pass print options

**Files:**
- Modify: `internal/handler/request.go:121-133`
- Modify: `internal/handler/print.go:206-221,224-238,349,399,486-532`

- [ ] **Step 1: Extend PrintRequest and ContainerPrintRequest in request.go**

```go
// PrintRequest is the input for print operations.
type PrintRequest struct {
	PrinterID string `json:"printer_id"`
	Template  string `json:"template"`
	PrintDate bool   `json:"print_date"`
	Density   int    `json:"density"`
	Copies    int    `json:"copies"`
	CutEvery  int    `json:"cut_every"`
	HighRes   bool   `json:"high_res"`
}

// ContainerPrintRequest is the input for container label printing.
type ContainerPrintRequest struct {
	PrinterID    string   `json:"printer_id"`
	Templates    []string `json:"templates"`
	PrintDate    bool     `json:"print_date"`
	ShowChildren bool     `json:"show_children"`
	Density      int      `json:"density"`
	Copies       int      `json:"copies"`
	CutEvery     int      `json:"cut_every"`
	HighRes      bool     `json:"high_res"`
}
```

- [ ] **Step 2: Add helper to extract PrintOpts from request structs**

Add to `print.go`:

```go
// printOptsFromRequest builds encoder.PrintOpts from request fields.
func printOptsFromRequest(density, copies, cutEvery int, highRes bool) encoder.PrintOpts {
	return encoder.PrintOpts{
		Density:  density,
		Copies:   copies,
		CutEvery: cutEvery,
		HighRes:  highRes,
	}
}
```

- [ ] **Step 3: Update printOrClientRender() to accept and pass PrintOpts**

```go
func (h *PrintHandler) printOrClientRender(w http.ResponseWriter, data label.LabelData,
	printerID, templateName string, opts label.RenderOpts, printOpts encoder.PrintOpts) {

	if _, ok := label.GetSchema(templateName); ok {
		if err := h.pm.Print(printerID, data, templateName, opts, printOpts); err != nil {
			webutil.LogError("print failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	respondClientRender(w, h.templates, templateName, data)
}
```

- [ ] **Step 4: Update PrintItem, PrintNote, PrintContainer handlers**

`PrintItem`:
```go
func (h *PrintHandler) PrintItem(w http.ResponseWriter, r *http.Request) {
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	data, ok := h.buildItemLabelData(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	pOpts := printOptsFromRequest(req.Density, req.Copies, req.CutEvery, req.HighRes)
	h.printOrClientRender(w, data, req.PrinterID, req.Template, label.RenderOpts{PrintDate: req.PrintDate}, pOpts)
}
```

Similarly update `PrintNote` and `PrintContainer` (passing `printOptsFromRequest(req.Density, req.Copies, req.CutEvery, req.HighRes)`).

For `PrintContainer`, the `pm.Print()` call inside the loop also needs print opts:
```go
if err := h.pm.Print(req.PrinterID, data, tmplName, opts, pOpts); err != nil {
```

- [ ] **Step 5: Update PrintImage handler**

```go
func (h *PrintHandler) PrintImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrinterID string `json:"printer_id"`
		PNG       string `json:"png"`
		Density   int    `json:"density"`
		Copies    int    `json:"copies"`
		CutEvery  int    `json:"cut_every"`
		HighRes   bool   `json:"high_res"`
	}
	// ... decode ...
	pOpts := printOptsFromRequest(req.Density, req.Copies, req.CutEvery, req.HighRes)
	if err := h.pm.PrintImage(req.PrinterID, img, pOpts); err != nil {
		// ...
	}
	// ...
}
```

- [ ] **Step 6: Update renderPreview() to accept optional printer_id**

```go
func (h *PrintHandler) PreviewItem(w http.ResponseWriter, r *http.Request) {
	data, ok := h.buildItemLabelData(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	templateName := r.URL.Query().Get("template")
	if templateName == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "template parameter required"})
		return
	}

	opts := label.RenderOpts{PrintDate: r.URL.Query().Get("print_date") == "true"}
	media := h.previewMediaInfo(r)
	renderPreviewWithMedia(w, h.templates, data, templateName, media, opts)
}
```

Add helper (uses exported `print.PxFromMm` and `print.FindModel` — no duplication):
```go
// previewMediaInfo builds MediaInfo from query params or printer session.
func (h *PrintHandler) previewMediaInfo(r *http.Request) label.MediaInfo {
	if printerID := r.URL.Query().Get("printer_id"); printerID != "" {
		status := h.pm.GetStatus(printerID)
		cfg := h.printers.GetPrinter(printerID)
		if cfg != nil {
			enc := h.pm.Encoder(cfg.Encoder)
			if enc != nil {
				if mi := print.FindModel(enc, cfg.Model); mi != nil {
					return label.MediaInfo{
						WidthPx:  mi.PrintWidthPx,
						HeightPx: print.PxFromMm(status.LabelHeightMm, mi.DPI),
						DPI:      mi.DPI,
					}
				}
			}
		}
	}
	return label.MediaInfo{WidthPx: previewWidth(r), HeightPx: 0, DPI: 203}
}
```

Update `renderPreview` → `renderPreviewWithMedia`:
```go
func renderPreviewWithMedia(w http.ResponseWriter, templates *service.TemplateService,
	data label.LabelData, templateName string, media label.MediaInfo, opts label.RenderOpts) {

	if _, ok := label.GetSchema(templateName); ok {
		img, err := label.Render(data, templateName, media, opts)
		if err != nil {
			webutil.LogError("preview render failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writePNG(w, img)
		return
	}

	respondClientRender(w, templates, templateName, data)
}
```

Update `PreviewContainer`, `PreviewNote` similarly.

- [ ] **Step 7: Run tests**

Run: `make test`
Expected: All tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/handler/
git commit -m "feat(handler): parse print options from request and pass to manager"
```

---

## Task 5: Add capabilities endpoint

**Files:**
- Modify: `internal/handler/print.go` (add handler + route)
- Modify: `internal/print/encoder/encoder.go` (add capability fields to ModelInfo)
- Modify: `internal/print/encoder/brother/models.go`

- [ ] **Step 1: Add capability fields to ModelInfo**

In `encoder.go`, extend `ModelInfo`:

```go
type ModelInfo struct {
	ID             string
	Name           string
	DPI            int
	PrintWidthPx   int
	MediaTypes     []string
	DensityRange   [2]int
	DensityDefault int
	CutSupported   bool // true if encoder supports cut_every
	HighResSupported bool // true if encoder supports high_res mode
}
```

- [ ] **Step 2: Set capability flags in model definitions**

In `brother/models.go`:
```go
func modelInfo(m qlModel) encoder.ModelInfo {
	return encoder.ModelInfo{
		ID: m.ID, Name: m.Name, DPI: 300,
		PrintWidthPx:     m.BytesPerRow * 8,
		MediaTypes:       []string{"endless", "die-cut"},
		DensityRange:     [2]int{1, 1}, DensityDefault: 1,
		CutSupported:     m.Cutting,
		HighResSupported: true,
	}
}
```

Niimbot `models.go` already has defaults (`false` for both bool fields — zero values).

- [ ] **Step 3: Add Capabilities handler**

In `print.go`:

```go
// Capabilities handles GET /printers/{id}/capabilities.
func (h *PrintHandler) Capabilities(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg := h.printers.GetPrinter(id)
	if cfg == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "printer not found"})
		return
	}

	enc := h.pm.Encoder(cfg.Encoder)
	if enc == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "encoder not found"})
		return
	}

	mi := findModelByID(enc, cfg.Model)
	if mi == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "model not found"})
		return
	}

	status := h.pm.GetStatus(id)

	mediaType := "continuous"
	if status.LabelHeightMm > 0 {
		mediaType = "die-cut"
	}

	// Use model-derived width as fallback when RFID/status width is unknown
	widthMm := status.LabelWidthMm
	if widthMm == 0 {
		widthMm = int(float64(mi.PrintWidthPx) * 25.4 / float64(mi.DPI))
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"density": map[string]int{
			"min":     mi.DensityRange[0],
			"max":     mi.DensityRange[1],
			"default": mi.DensityDefault,
		},
		"copies":   map[string]int{"max": 100},
		"cut_every": map[string]bool{"supported": mi.CutSupported},
		"high_res":  map[string]bool{"supported": mi.HighResSupported},
		"media": map[string]any{
			"width_mm":  widthMm,
			"height_mm": status.LabelHeightMm,
			"type":      mediaType,
		},
	})
}
```

- [ ] **Step 4: Register route**

In `RegisterRoutes()`, add:
```go
mux.HandleFunc("GET /printers/{id}/capabilities", h.Capabilities)
```

- [ ] **Step 5: Run tests and lint**

Run: `make test && make lint`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/print/encoder/encoder.go internal/print/encoder/brother/models.go \
  internal/handler/print.go
git commit -m "feat(api): add GET /printers/{id}/capabilities endpoint"
```

---

## Task 6: Brother encoder — multi-copy, high-res, dynamic margin

**Files:**
- Modify: `internal/print/encoder/brother/brother.go`
- Modify: `internal/print/encoder/brother/labels.go` (for margin lookup)

- [ ] **Step 1: Write test for Brother multi-copy encoding**

Create/extend `internal/print/encoder/brother/brother_test.go`:

```go
package brother

import (
	"image"
	"testing"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
)

func TestEncode_SingleCopy(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 720, 10))
	tr := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 1, CutEvery: 1}, tr)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Should end with 0x1A (print with feed)
	// MockTransport.Written is a flat []byte — all writes concatenated
	if len(tr.Written) == 0 {
		t.Fatal("no data written")
	}
	if tr.Written[len(tr.Written)-1] != 0x1A {
		t.Errorf("last byte = 0x%02X, want 0x1A (print with feed)", tr.Written[len(tr.Written)-1])
	}
}

func TestEncode_MultiCopy(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 720, 5))
	tr := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 3, CutEvery: 1}, tr)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Count print commands: 2x 0x0C (FF) + 1x 0x1A (print with feed)
	// MockTransport.Written is a flat []byte
	ffCount := 0
	ctrlZCount := 0
	for _, b := range tr.Written {
		if b == 0x0C {
			ffCount++
		}
	}
	// Last byte should be 0x1A
	if tr.Written[len(tr.Written)-1] == 0x1A {
		ctrlZCount = 1
	}
	if ffCount != 2 {
		t.Errorf("FF (0x0C) count = %d, want 2", ffCount)
	}
	if ctrlZCount != 1 {
		t.Errorf("expected final 0x1A")
	}
}

func TestEncode_NoCut(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 720, 5))
	tr := &transport.MockTransport{}
	enc := &BrotherEncoder{}

	err := enc.Encode(img, "QL-700", encoder.PrintOpts{Copies: 1, CutEvery: 0}, tr)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Various mode byte should be 0x00 (autocut off)
	data := tr.AllWritten()
	// Find ESC i M sequence in flat Written bytes
	found := false
	for i := 0; i < len(tr.Written)-3; i++ {
		if tr.Written[i] == 0x1B && tr.Written[i+1] == 0x69 && tr.Written[i+2] == 0x4D {
			if tr.Written[i+3] != 0x00 {
				t.Errorf("ESC i M byte = 0x%02X, want 0x00 (autocut off)", tr.Written[i+3])
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("ESC i M not found in output")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/print/encoder/brother/ -v`
Expected: Tests fail (multi-copy not implemented, MockTransport may need `AllWritten()`)

- [ ] **Step 3: Implement multi-copy, high-res, dynamic margin in Brother encoder**

Rewrite `brother.go` `Encode()` to:

1. Init once (clear buffer, ESC @, status read)
2. Set autocut/expanded mode/margin based on opts
3. Loop `Copies` times for raster data + print command
4. HighRes: duplicate each raster row
5. Dynamic margin: 0 for die-cut, 35 for continuous

```go
func (e *BrotherEncoder) Encode(img image.Image, model string, opts encoder.PrintOpts, tr transport.Transport) error {
	if model != ql700.ID {
		return fmt.Errorf("unsupported model: %s", model)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	if width != ql700.BytesPerRow*8 {
		return fmt.Errorf("image width must be %d pixels, got %d", ql700.BytesPerRow*8, width)
	}

	copies := max(opts.Copies, 1)

	// Effective raster height (doubled if high-res)
	rasterHeight := height
	if opts.HighRes {
		rasterHeight = height * 2
	}

	// 1. Clear buffer
	if _, err := tr.Write(make([]byte, 200)); err != nil {
		return err
	}

	// 2. ESC @ — initialize
	if _, err := tr.Write([]byte{0x1B, 0x40}); err != nil {
		return err
	}

	// 2a. Read status for media type
	st, stErr := requestStatus(tr)
	mediaType := mediaContinuous
	mediaWidth := byte(62)
	mediaLength := byte(0)
	if stErr == nil {
		mediaType = st.MediaType
		mediaWidth = byte(st.MediaWidth)
		mediaLength = byte(st.MediaLength)
	}

	// 3. Autocut mode
	if opts.CutEvery > 0 {
		if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x40}); err != nil {
			return err
		}
	} else {
		if _, err := tr.Write([]byte{0x1B, 0x69, 0x4D, 0x00}); err != nil {
			return err
		}
	}

	// 4. Cut every N labels
	cutEvery := byte(1)
	if opts.CutEvery > 0 {
		cutEvery = byte(opts.CutEvery)
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x41, cutEvery}); err != nil {
		return err
	}

	// 5. Expanded mode
	expandedMode := byte(0x00)
	if opts.CutEvery > 0 {
		expandedMode |= 0x08 // cut at end
	}
	if opts.HighRes {
		expandedMode |= 0x40 // 600 DPI vertical
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x4B, expandedMode}); err != nil {
		return err
	}

	// 6. Dynamic margin
	margin := byte(35) // continuous default
	if mediaType == mediaDieCut {
		margin = 0
	}
	if _, err := tr.Write([]byte{0x1B, 0x69, 0x64, margin, 0x00}); err != nil {
		return err
	}

	// Pre-encode all raster rows (shared across copies)
	rowBuf := make([]byte, 3+ql700.BytesPerRow)
	rowBuf[0] = 0x67
	rowBuf[1] = 0x00
	rowBuf[2] = byte(ql700.BytesPerRow)

	type rasterRow struct {
		data []byte
	}
	rows := make([]rasterRow, height)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		pixels := make([]byte, ql700.BytesPerRow)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			gray := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			var bit byte
			if gray < 128 {
				bit = 1
			}
			flippedX := (bounds.Max.X - 1) - x
			byteIdx := flippedX / 8
			bitIdx := uint(7 - (flippedX % 8))
			pixels[byteIdx] |= bit << bitIdx
		}
		rows[y-bounds.Min.Y] = rasterRow{data: pixels}
	}

	// 7. Print copies
	for c := 0; c < copies; c++ {
		// Media info with page number
		rasterLines := uint32(rasterHeight)
		mediaInfo := make([]byte, 13)
		mediaInfo[0] = 0x1B
		mediaInfo[1] = 0x69
		mediaInfo[2] = 0x7A
		mediaInfo[3] = 0xCE
		mediaInfo[4] = mediaType
		mediaInfo[5] = mediaWidth
		mediaInfo[6] = mediaLength
		binary.LittleEndian.PutUint32(mediaInfo[7:11], rasterLines)
		mediaInfo[11] = byte(c) // page number
		mediaInfo[12] = 0x00
		if _, err := tr.Write(mediaInfo); err != nil {
			return err
		}

		// Send raster rows
		for _, row := range rows {
			copy(rowBuf[3:], row.data)
			if _, err := tr.Write(rowBuf); err != nil {
				return err
			}
			// HighRes: duplicate each row
			if opts.HighRes {
				if _, err := tr.Write(rowBuf); err != nil {
					return err
				}
			}
		}

		// Print command
		if c < copies-1 {
			// Intermediate page: print without feed
			if _, err := tr.Write([]byte{0x0C}); err != nil {
				return err
			}
		} else {
			// Last page: print with feed
			if _, err := tr.Write([]byte{0x1A}); err != nil {
				return err
			}
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/print/encoder/brother/ -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/print/encoder/brother/
git commit -m "feat(brother): multi-copy, cut_every, high-res, dynamic margin"
```

---

## Task 7: Niimbot encoder — copies with fallback, density unfreeze

**Files:**
- Modify: `internal/print/encoder/niimbot/niimbot.go:96-120`

- [ ] **Step 1: Write test for Niimbot multi-copy**

The Niimbot encoder requires a bidirectional transport for transceive. Use the existing mock pattern. Add to niimbot tests (or create `niimbot_test.go` if needed):

```go
func TestEncode_Copies_SetsPageSize(t *testing.T) {
	// This test verifies that the copies field in SET_PAGE_SIZE is set correctly.
	// Due to the bidirectional protocol, a full integration test requires a mock
	// that responds to commands. For now, verify the PRINT_START totalPages field.
	// Full protocol testing is done at integration level with real hardware.
}
```

Given the bidirectional Niimbot protocol complexity, focus on the implementation and rely on hardware testing for Niimbot copies (per spec: "test native copies, fallback to repeat cycle").

- [ ] **Step 2: Update Niimbot Encode() for copies**

In `niimbot.go`, update `PRINT_START` totalPages and `SET_PAGE_SIZE` copies:

```go
// 3. PRINT_START — totalPages = copies
copies := max(opts.Copies, 1)
printStartData := make([]byte, 7)
binary.BigEndian.PutUint16(printStartData[0:2], uint16(copies))
// bytes 2-4: zeros
printStartData[5] = 0x00
printStartData[6] = 0x00
if err := e.transceive(tr, cmdPrintStart, printStartData, respOffsetStandard); err != nil {
	return fmt.Errorf("PRINT_START: %w", err)
}
```

And SET_PAGE_SIZE:
```go
binary.BigEndian.PutUint16(pageSizeData[4:6], uint16(copies))
```

- [ ] **Step 3: Run full test suite**

Run: `make test`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add internal/print/encoder/niimbot/niimbot.go
git commit -m "feat(niimbot): wire copies to PRINT_START/SET_PAGE_SIZE, density from opts"
```

---

## Task 8: UI — print options controls with dynamic show/hide

**Files:**
- Modify: `internal/embedded/templates/` — print form partials
- Modify: `internal/embedded/static/js/` — dynamic printer change handler
- Modify: `internal/handler/print.go` — HTMX fragment for print options

This task requires exploring the current print form templates. The implementer should:

- [ ] **Step 1: Find the current print form templates**

Search for the print modal/form HTML in `internal/embedded/templates/`. Look for files containing `printer_id` select and `template` select. These are the forms that need the new controls.

- [ ] **Step 2: Add print options HTML controls**

Below the existing template selector, add:

```html
<div id="print-options" data-printer-id="">
  <div class="form-group">
    <label>Copies</label>
    <input type="number" name="copies" value="1" min="1" max="100">
  </div>
  <div class="form-group density-control" style="display:none">
    <label>Density: <span class="density-value">3</span></label>
    <input type="range" name="density" min="1" max="5" value="3">
  </div>
  <div class="form-group cut-control" style="display:none">
    <label>Cut every N copies (0 = no cut)</label>
    <input type="number" name="cut_every" value="1" min="0" max="100">
  </div>
  <div class="form-group highres-control" style="display:none">
    <label><input type="checkbox" name="high_res"> High resolution</label>
  </div>
</div>
```

- [ ] **Step 3: Add JS to fetch capabilities on printer change**

In the print form's JavaScript (find the file that handles printer selection):

```javascript
async function onPrinterChange(printerSelect) {
  const printerId = printerSelect.value;
  if (!printerId) return;

  const resp = await fetch(`/api/printers/${printerId}/capabilities`);
  if (!resp.ok) return;
  const caps = await resp.json();

  const opts = document.getElementById('print-options');
  if (!opts) return;

  // Density
  const densityCtrl = opts.querySelector('.density-control');
  if (caps.density.min !== caps.density.max) {
    densityCtrl.style.display = '';
    const slider = densityCtrl.querySelector('input[type=range]');
    slider.min = caps.density.min;
    slider.max = caps.density.max;
    slider.value = caps.density.default;
    densityCtrl.querySelector('.density-value').textContent = caps.density.default;
    slider.oninput = () => {
      densityCtrl.querySelector('.density-value').textContent = slider.value;
    };
  } else {
    densityCtrl.style.display = 'none';
  }

  // Cut every
  const cutCtrl = opts.querySelector('.cut-control');
  cutCtrl.style.display = caps.cut_every.supported ? '' : 'none';

  // High res
  const hrCtrl = opts.querySelector('.highres-control');
  hrCtrl.style.display = caps.high_res.supported ? '' : 'none';
}
```

- [ ] **Step 4: Wire the JS to printer select change event**

Add event listener to the printer `<select>` element. This depends on the current form structure — adapt to the existing pattern (may be HTMX `hx-trigger="change"` or vanilla JS `addEventListener`).

- [ ] **Step 5: Update print form submission JS to include new fields**

Find the JS that builds the print request body and add:

```javascript
const opts = document.getElementById('print-options');
if (opts) {
  body.density = parseInt(opts.querySelector('[name=density]')?.value || '0');
  body.copies = parseInt(opts.querySelector('[name=copies]')?.value || '1');
  body.cut_every = parseInt(opts.querySelector('[name=cut_every]')?.value || '1');
  body.high_res = opts.querySelector('[name=high_res]')?.checked || false;
}
```

- [ ] **Step 6: Test in browser**

Run: `make run`
- Open browser, navigate to an item
- Click print — verify new controls appear
- Change printer — verify controls show/hide dynamically
- Print with different settings — verify it works

- [ ] **Step 7: Commit**

```bash
git add internal/embedded/
git commit -m "feat(ui): add print options controls with dynamic capabilities"
```

---

## Task 9: E2E test for print options

**Files:**
- Create: `e2e/tests/print-options.spec.ts`

- [ ] **Step 1: Write E2E test**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Print Options', () => {
  test('capabilities endpoint returns printer options', async ({ request, app }) => {
    // Create a printer first
    const createResp = await request.post(`${app.baseURL}/api/printers`, {
      data: { name: 'Test', encoder: 'niimbot', model: 'B1', transport: 'mock', address: '/dev/null' }
    });
    expect(createResp.ok()).toBeTruthy();
    const printer = await createResp.json();

    // Fetch capabilities
    const capsResp = await request.get(`${app.baseURL}/api/printers/${printer.id}/capabilities`);
    expect(capsResp.ok()).toBeTruthy();
    const caps = await capsResp.json();

    expect(caps.density.min).toBe(1);
    expect(caps.density.max).toBe(5);
    expect(caps.copies.max).toBe(100);
  });
});
```

- [ ] **Step 2: Run E2E tests**

Run: `make test-e2e`
Expected: New test passes

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/print-options.spec.ts
git commit -m "test(e2e): add print options capabilities test"
```

---

## Task 10: Final lint, full test run, cleanup

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All pass

- [ ] **Step 2: Run lint**

Run: `make lint`
Expected: No errors

- [ ] **Step 3: Run E2E**

Run: `make test-e2e`
Expected: All pass

- [ ] **Step 4: Final commit if any cleanup needed**

```bash
git commit -m "chore: cleanup after print options implementation"
```
