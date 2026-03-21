package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

type Server struct {
	store          *store.Store
	printerManager *print.PrinterManager
	translations   *webutil.Translations
	inventory      *service.InventoryService
	bulk           *service.BulkService
	tags           *service.TagService
	search         *service.SearchService
	printers       *service.PrinterService
}

func NewServer(s *store.Store, pm *print.PrinterManager, tr *webutil.Translations,
	inventory *service.InventoryService, bulk *service.BulkService,
	tags *service.TagService, search *service.SearchService,
	printers *service.PrinterService) *Server {
	return &Server{
		store:          s,
		printerManager: pm,
		translations:   tr,
		inventory:      inventory,
		bulk:           bulk,
		tags:           tags,
		search:         search,
		printers:       printers,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/containers", s.HandleContainers)
	mux.HandleFunc("POST /api/containers", s.HandleContainerCreate)
	mux.HandleFunc("GET /api/containers/{id}", s.HandleContainer)
	mux.HandleFunc("PUT /api/containers/{id}", s.HandleContainerUpdate)
	mux.HandleFunc("DELETE /api/containers/{id}", s.HandleContainerDelete)
	mux.HandleFunc("GET /api/containers/{id}/items", s.HandleContainerItems)

	mux.HandleFunc("GET /api/items/{id}", s.HandleItem)
	mux.HandleFunc("POST /api/items", s.HandleItemCreate)
	mux.HandleFunc("PUT /api/items/{id}", s.HandleItemUpdate)
	mux.HandleFunc("DELETE /api/items/{id}", s.HandleItemDelete)

	mux.HandleFunc("PATCH /api/containers/{id}/move", s.HandleContainerMove)
	mux.HandleFunc("PATCH /api/items/{id}/move", s.HandleItemMove)

	// Tags
	mux.HandleFunc("GET /api/tags", s.HandleTags)
	mux.HandleFunc("POST /api/tags", s.HandleTagCreate)
	mux.HandleFunc("GET /api/tags/{id}", s.HandleTag)
	mux.HandleFunc("PUT /api/tags/{id}", s.HandleTagUpdate)
	mux.HandleFunc("DELETE /api/tags/{id}", s.HandleTagDelete)
	mux.HandleFunc("PATCH /api/tags/{id}/move", s.HandleTagMove)
	mux.HandleFunc("GET /api/tags/{id}/descendants", s.HandleTagDescendants)
	mux.HandleFunc("POST /api/items/{id}/tags", s.HandleItemTagAdd)
	mux.HandleFunc("DELETE /api/items/{id}/tags/{tag_id}", s.HandleItemTagRemove)
	mux.HandleFunc("POST /api/containers/{id}/tags", s.HandleContainerTagAdd)
	mux.HandleFunc("DELETE /api/containers/{id}/tags/{tag_id}", s.HandleContainerTagRemove)

	// Bulk
	mux.HandleFunc("POST /api/bulk/move", s.HandleBulkMove)
	mux.HandleFunc("POST /api/bulk/delete", s.HandleBulkDelete)
	mux.HandleFunc("POST /api/bulk/tags", s.HandleBulkTags)

	// Search
	mux.HandleFunc("GET /api/search", s.HandleSearch)

	mux.HandleFunc("GET /api/i18n/{lang}", s.HandleI18n)

	mux.HandleFunc("GET /api/export/json", s.HandleExportJSON)
	mux.HandleFunc("GET /api/export/csv", s.HandleExportCSV)

	mux.HandleFunc("GET /api/printers", s.HandlePrinters)
	mux.HandleFunc("POST /api/printers", s.HandlePrinterCreate)
	mux.HandleFunc("DELETE /api/printers/{id}", s.HandlePrinterDelete)
	mux.HandleFunc("GET /api/encoders", s.HandleEncoders)
	mux.HandleFunc("POST /api/items/{id}/print", s.HandlePrint)
	mux.HandleFunc("GET /api/printers/status", s.HandlePrinterStatuses)
	mux.HandleFunc("GET /api/printers/{id}/status", s.HandlePrinterStatus)
	mux.HandleFunc("POST /api/printers/{id}/connect", s.HandlePrinterConnect)
	mux.HandleFunc("POST /api/printers/{id}/disconnect", s.HandlePrinterDisconnect)
	mux.HandleFunc("GET /api/printers/events", s.HandlePrinterEvents)
	s.registerBluetoothRoutes(mux)
}

type upsertContainerRequest struct {
	ParentID    string `json:"parent_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type upsertItemRequest struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
}

func (s *Server) HandleContainers(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	if parentID != "" || r.URL.Query().Has("parent_id") {
		webutil.JSON(w, http.StatusOK, s.inventory.ContainerChildren(parentID))
		return
	}
	webutil.JSON(w, http.StatusOK, s.store.AllContainers())
}

func (s *Server) HandleContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := s.inventory.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"container": container,
		"children":  s.inventory.ContainerChildren(id),
		"path":      s.inventory.ContainerPath(id),
	})
}

func (s *Server) HandleContainerItems(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := s.inventory.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"container": container,
		"items":     s.inventory.ContainerItems(id),
		"path":      s.inventory.ContainerPath(id),
	})
}

func (s *Server) HandleContainerCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertContainerRequest{
		ParentID:    r.FormValue("parent_id"),   //nolint:gosec // G120: internal tool, no untrusted input
		Name:        r.FormValue("name"),        //nolint:gosec // G120: internal tool, no untrusted input
		Description: r.FormValue("description"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if strings.TrimSpace(req.Name) == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	container, err := s.inventory.CreateContainer(req.ParentID, req.Name, req.Description)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusCreated, container)
}

func (s *Server) HandleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	req := upsertContainerRequest{
		Name:        r.FormValue("name"),        //nolint:gosec // G120: internal tool, no untrusted input
		Description: r.FormValue("description"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	container, err := s.inventory.UpdateContainer(r.PathValue("id"), req.Name, req.Description)
	if err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, container)
}

func (s *Server) HandleContainerDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.inventory.DeleteContainer(r.PathValue("id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) HandleItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item := s.inventory.GetItem(id)
	if item == nil {
		http.NotFound(w, r)
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]any{
		"item": item,
		"path": s.inventory.ContainerPath(item.ContainerID),
	})
}

func (s *Server) HandleItemCreate(w http.ResponseWriter, r *http.Request) {
	req := upsertItemRequest{
		ContainerID: r.FormValue("container_id"), //nolint:gosec // G120: internal tool, no untrusted input
		Name:        r.FormValue("name"),         //nolint:gosec // G120: internal tool, no untrusted input
		Description: r.FormValue("description"),  //nolint:gosec // G120: internal tool, no untrusted input
	}
	if qStr := r.FormValue("quantity"); qStr != "" { //nolint:gosec // G120: internal tool, no untrusted input
		if q, err := strconv.Atoi(qStr); err == nil {
			req.Quantity = q
		}
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if strings.TrimSpace(req.Name) == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.ContainerID == "" {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "container_id is required"})
		return
	}

	item, err := s.inventory.CreateItem(req.ContainerID, req.Name, req.Description, req.Quantity)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusCreated, item)
}

func (s *Server) HandleItemUpdate(w http.ResponseWriter, r *http.Request) {
	req := upsertItemRequest{
		Name:        r.FormValue("name"),        //nolint:gosec // G120: internal tool, no untrusted input
		Description: r.FormValue("description"), //nolint:gosec // G120: internal tool, no untrusted input
	}
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	item, err := s.inventory.UpdateItem(r.PathValue("id"), req.Name, req.Description)
	if err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, item)
}

func (s *Server) HandleItemDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.inventory.DeleteItem(r.PathValue("id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type moveContainerRequest struct {
	ParentID string `json:"parent_id"`
}

type moveItemRequest struct {
	ContainerID string `json:"container_id"`
}

func (s *Server) HandleContainerMove(w http.ResponseWriter, r *http.Request) {
	var req moveContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.inventory.MoveContainer(r.PathValue("id"), req.ParentID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemMove(w http.ResponseWriter, r *http.Request) {
	var req moveItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.inventory.MoveItem(r.PathValue("id"), req.ContainerID); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleI18n returns a merged translation map for the requested language.
func (s *Server) HandleI18n(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	merged := s.translations.Merged(lang)
	webutil.JSON(w, http.StatusOK, merged)
}

func (s *Server) HandleExportJSON(w http.ResponseWriter, r *http.Request) {
	containers, items := s.store.ExportData()
	webutil.JSON(w, http.StatusOK, map[string]any{
		"containers": containers,
		"items":      items,
	})
}

func (s *Server) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{"item_id", "item_name", "item_description", "container_path", "created_at"})

	items := s.store.AllItems()
	for _, item := range items {
		path := s.inventory.ContainerPath(item.ContainerID)
		pathStr := webutil.FormatContainerPath(path, " -> ")

		_ = cw.Write([]string{
			item.ID,
			item.Name,
			item.Description,
			pathStr,
			item.CreatedAt.Format(time.RFC3339),
		})
	}
}

type addPrinterRequest struct {
	Name      string `json:"name"`
	Encoder   string `json:"encoder"`
	Model     string `json:"model"`
	Transport string `json:"transport"`
	Address   string `json:"address"`
}

type printRequest struct {
	PrinterID string `json:"printer_id"`
	Template  string `json:"template"`
}

func (s *Server) HandlePrinters(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, s.printers.AllPrinters())
}

func (s *Server) HandlePrinterCreate(w http.ResponseWriter, r *http.Request) {
	var req addPrinterRequest
	if isJSONBody(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.Name = r.FormValue("name")           //nolint:gosec // G120: internal tool, no untrusted input
		req.Encoder = r.FormValue("encoder")     //nolint:gosec // G120: internal tool, no untrusted input
		req.Model = r.FormValue("model")         //nolint:gosec // G120: internal tool, no untrusted input
		req.Transport = r.FormValue("transport") //nolint:gosec // G120: internal tool, no untrusted input
		req.Address = r.FormValue("address")     //nolint:gosec // G120: internal tool, no untrusted input
	}

	printer, err := s.printers.AddPrinter(req.Name, req.Encoder, req.Model, req.Transport, req.Address)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusCreated, printer)
}

func (s *Server) HandlePrinterDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.printers.DeletePrinter(r.PathValue("id")); err != nil {
		webutil.WriteStoreErrorJSON(w, err)
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) HandleEncoders(w http.ResponseWriter, r *http.Request) {
	type encoderInfo struct {
		Name   string              `json:"name"`
		Models []map[string]string `json:"models"`
	}

	var result []encoderInfo
	for name, enc := range s.printerManager.AvailableEncoders() {
		info := encoderInfo{Name: name}
		for _, m := range enc.Models() {
			info.Models = append(info.Models, map[string]string{
				"id":   m.ID,
				"name": m.Name,
			})
		}
		result = append(result, info)
	}
	webutil.JSON(w, http.StatusOK, result)
}

func (s *Server) HandlePrint(w http.ResponseWriter, r *http.Request) {
	var req printRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	item := s.inventory.GetItem(r.PathValue("id"))
	if item == nil {
		http.NotFound(w, r)
		return
	}

	path := s.inventory.ContainerPath(item.ContainerID)
	data := label.LabelData{
		Name:        item.Name,
		Description: item.Description,
		Location:    webutil.FormatContainerPath(path, " → "),
		QRContent:   "/item/" + item.ID,
		BarcodeID:   item.ID,
	}

	if err := s.printerManager.Print(req.PrinterID, data, req.Template); err != nil {
		webutil.LogError("print failed: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandlePrinterStatuses(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, s.printerManager.AllStatuses())
}

func (s *Server) HandlePrinterStatus(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, s.printerManager.GetStatus(r.PathValue("id")))
}

func (s *Server) HandlePrinterConnect(w http.ResponseWriter, r *http.Request) {
	if err := s.printerManager.ConnectPrinter(r.PathValue("id")); err != nil {
		webutil.LogError("connect printer: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandlePrinterDisconnect(w http.ResponseWriter, r *http.Request) {
	s.printerManager.DisconnectPrinter(r.PathValue("id"))
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandlePrinterEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := s.printerManager.SubscribeSSE()
	defer s.printerManager.UnsubscribeSSE(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			data, _ := json.Marshal(evt)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func isJSONBody(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}
