package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	data, _ := s.containerViewModel("")
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainer(w http.ResponseWriter, r *http.Request) {
	data, ok := s.containerViewModel(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerCreate(w http.ResponseWriter, r *http.Request) {
	parentID := r.FormValue("parent_id")      //nolint:gosec // G120: internal tool, no untrusted input
	name := r.FormValue("name")               //nolint:gosec // G120: internal tool, no untrusted input
	description := r.FormValue("description") //nolint:gosec // G120: internal tool, no untrusted input

	container := s.store.CreateContainer(parentID, name, description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	data, _ := s.containerViewModel(container.ID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.FormValue("name")               //nolint:gosec // G120: internal tool, no untrusted input
	description := r.FormValue("description") //nolint:gosec // G120: internal tool, no untrusted input

	_, err := s.store.UpdateContainer(id, name, description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(id)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	container := s.store.GetContainer(id)
	var parentID string
	if container != nil {
		parentID = container.ParentID
	}

	err := s.store.DeleteContainer(id)
	if err != nil {
		if err == store.ErrContainerNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(parentID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleItem(w http.ResponseWriter, r *http.Request) {
	data, ok := s.itemDetailViewModel(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "item", data)
}

func (s *Server) HandleItemCreate(w http.ResponseWriter, r *http.Request) {
	containerID := r.FormValue("container_id") //nolint:gosec // G120: internal tool, no untrusted input
	name := r.FormValue("name")                //nolint:gosec // G120: internal tool, no untrusted input
	description := r.FormValue("description")  //nolint:gosec // G120: internal tool, no untrusted input

	s.store.CreateItem(containerID, name, description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, ok := s.containerViewModel(containerID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "containers", data)
}

func (s *Server) HandleItemUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.FormValue("name")               //nolint:gosec // G120: internal tool, no untrusted input
	description := r.FormValue("description") //nolint:gosec // G120: internal tool, no untrusted input

	item, err := s.store.UpdateItem(id, name, description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.itemDetailViewModel(item.ID)
	s.render(w, r, "item", data)
}

func (s *Server) HandleItemDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	item := s.store.GetItem(id)
	var containerID string
	if item != nil {
		containerID = item.ContainerID
	}

	err := s.store.DeleteItem(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(containerID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newParentID := r.FormValue("parent_id") //nolint:gosec // G120: internal tool, no untrusted input

	if err := s.store.MoveContainer(id, newParentID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newContainerID := r.FormValue("container_id") //nolint:gosec // G120: internal tool, no untrusted input

	if err := s.store.MoveItem(id, newContainerID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandlePrinters(w http.ResponseWriter, r *http.Request) {
	data := s.printersViewModel()
	s.render(w, r, "printers", data)
}

func (s *Server) HandlePrinterCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")           //nolint:gosec // G120: internal tool, no untrusted input
	enc := r.FormValue("encoder")         //nolint:gosec // G120: internal tool, no untrusted input
	model := r.FormValue("model")         //nolint:gosec // G120: internal tool, no untrusted input
	transport := r.FormValue("transport") //nolint:gosec // G120: internal tool, no untrusted input
	address := r.FormValue("address")     //nolint:gosec // G120: internal tool, no untrusted input

	s.store.AddPrinter(name, enc, model, transport, address)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data := s.printersViewModel()
	s.render(w, r, "printers", data)
}

func (s *Server) HandlePrinterDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.store.DeletePrinter(id); err != nil {
		if err == store.ErrPrinterNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data := s.printersViewModel()
	s.render(w, r, "printers", data)
}

func (s *Server) HandleItemPrint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	item := s.store.GetItem(id)
	if item == nil {
		webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	var req struct {
		PrinterID    string `json:"printer_id"`
		TemplateName string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	path := s.store.ContainerPath(item.ContainerID)
	pathParts := make([]string, 0, len(path))
	for _, c := range path {
		pathParts = append(pathParts, c.Name)
	}

	data := label.LabelData{
		Name:        item.Name,
		Description: item.Description,
		Location:    strings.Join(pathParts, " → "),
		QRContent:   fmt.Sprintf("/ui/items/%s", item.ID),
		BarcodeID:   item.ID,
	}

	// Check if this is a legacy template (server-side rendering) or designer template
	switch req.TemplateName {
	case "simple", "standard", "compact", "detailed":
		// Legacy templates: server-side rendering via label.Render()
		if err := s.printerManager.Print(req.PrinterID, data, req.TemplateName); err != nil {
			webutil.LogError("print failed: %v", err)
			webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		// Designer template: return template + item data for client-side rendering
		tmpl := s.store.GetTemplate(req.TemplateName)
		if tmpl == nil {
			webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		webutil.JSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"render":    "client",
			"template":  tmpl,
			"item_data": data,
		})
	}
}

func (s *Server) HandleTemplates(w http.ResponseWriter, r *http.Request) {
	activeTag := r.URL.Query().Get("tag")
	all := s.store.AllTemplates()

	tagSet := make(map[string]bool)
	for _, t := range all {
		for _, tag := range t.Tags {
			tagSet[tag] = true
		}
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	var filtered []store.Template
	if activeTag == "" {
		filtered = all
	} else {
		for _, t := range all {
			for _, tag := range t.Tags {
				if tag == activeTag {
					filtered = append(filtered, t)
					break
				}
			}
		}
	}

	s.render(w, r, "templates", TemplateListData{
		Templates: filtered,
		Tags:      tags,
		ActiveTag: activeTag,
	})
}

func (s *Server) HandleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.store.DeleteTemplate(id)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	s.HandleTemplates(w, r)
}

func (s *Server) HandleTemplateNew(w http.ResponseWriter, r *http.Request) {
	models := collectPrinterModels(s)
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(map[string]string{
		"name":        "Sample Item",
		"description": "A sample item for preview",
		"location":    "Warehouse > Shelf A",
		"qr_content":  "/ui/items/preview",
		"barcode_id":  "PREVIEW001",
	})

	s.render(w, r, "template-designer", DesignerData{
		Target:            "universal",
		Width:             62,
		Height:            29,
		TemplateJSON:      "[]",
		PrinterModels:     models,
		PrinterModelsJSON: string(modelsJSON),
		PreviewDataJSON:   string(previewJSON),
	})
}

func (s *Server) HandleTemplateEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl := s.store.GetTemplate(id)
	if tmpl == nil {
		http.NotFound(w, r)
		return
	}

	models := collectPrinterModels(s)
	modelsJSON, _ := json.Marshal(models)
	previewJSON, _ := json.Marshal(map[string]string{
		"name":        "Sample Item",
		"description": "A sample item for preview",
		"location":    "Warehouse > Shelf A",
		"qr_content":  "/ui/items/preview",
		"barcode_id":  "PREVIEW001",
	})

	width := tmpl.WidthMM
	height := tmpl.HeightMM
	if strings.HasPrefix(tmpl.Target, "printer:") {
		width = float64(tmpl.WidthPx)
		height = float64(tmpl.HeightPx)
	}

	s.render(w, r, "template-designer", DesignerData{
		TemplateID:        tmpl.ID,
		TemplateName:      tmpl.Name,
		TemplateTags:      strings.Join(tmpl.Tags, ", "),
		Target:            tmpl.Target,
		Width:             width,
		Height:            height,
		TemplateJSON:      tmpl.Elements,
		PrinterModels:     models,
		PrinterModelsJSON: string(modelsJSON),
		PreviewDataJSON:   string(previewJSON),
	})
}

func (s *Server) HandleTemplateSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Tags     []string `json:"tags"`
		Target   string   `json:"target"`
		WidthMM  float64  `json:"width_mm"`
		HeightMM float64  `json:"height_mm"`
		WidthPx  int      `json:"width_px"`
		HeightPx int      `json:"height_px"`
		Elements string   `json:"elements"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	tags := req.Tags
	id := r.PathValue("id")

	if id != "" {
		// Update existing
		tmpl := s.store.GetTemplate(id)
		if tmpl == nil {
			webutil.JSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		tmpl.Name = req.Name
		tmpl.Tags = tags
		tmpl.Target = req.Target
		if strings.HasPrefix(req.Target, "printer:") {
			tmpl.WidthPx = req.WidthPx
			tmpl.HeightPx = req.HeightPx
			tmpl.WidthMM = 0
			tmpl.HeightMM = 0
		} else {
			tmpl.WidthMM = req.WidthMM
			tmpl.HeightMM = req.HeightMM
			tmpl.WidthPx = 0
			tmpl.HeightPx = 0
		}
		tmpl.Elements = req.Elements
		tmpl.UpdatedAt = time.Now()
		s.store.SaveTemplate(*tmpl)
	} else {
		// Create new
		s.store.CreateTemplate(req.Name, tags, req.Target, req.WidthMM, req.HeightMM, req.WidthPx, req.HeightPx, req.Elements)
	}

	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandlePrintImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrinterID string `json:"printer_id"`
		PNG       string `json:"png"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Decode base64 PNG (format: "data:image/png;base64,XXXX")
	parts := strings.SplitN(req.PNG, ",", 2)
	if len(parts) != 2 {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid PNG data"})
		return
	}
	imgData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "base64 decode: " + err.Error()})
		return
	}

	img, err := png.Decode(bytes.NewReader(imgData))
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "png decode: " + err.Error()})
		return
	}

	if err := s.printerManager.PrintImage(req.PrinterID, img); err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleAssetUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil { // 1MB limit
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	asset, err := s.store.SaveAsset(header.Filename, header.Header.Get("Content-Type"), data)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]string{"id": asset.ID, "name": asset.Name})
}

func (s *Server) HandleAssetServe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	asset := s.store.GetAsset(id)
	if asset == nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.store.AssetData(id)
	if err != nil {
		http.Error(w, "asset read error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", asset.MimeType)
	_, _ = w.Write(data)
}

func (s *Server) HandleContainerItemsJSON(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	items := s.store.ContainerItems(id)

	var result []map[string]string
	for _, item := range items {
		path := s.store.ContainerPath(item.ContainerID)
		var parts []string
		for _, c := range path {
			parts = append(parts, c.Name)
		}
		result = append(result, map[string]string{
			"name": item.Name, "description": item.Description,
			"location": strings.Join(parts, " → "), "id": item.ID,
			"qr_url": "/ui/items/" + item.ID,
		})
	}
	webutil.JSON(w, http.StatusOK, result)
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var tags []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func collectPrinterModels(s *Server) []encoder.ModelInfo {
	var models []encoder.ModelInfo
	for _, enc := range s.printerManager.AvailableEncoders() {
		models = append(models, enc.Models()...)
	}
	return models
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
