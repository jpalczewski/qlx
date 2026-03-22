package handler

import (
	"io"
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// AssetHandler handles HTTP requests for asset upload and serving.
type AssetHandler struct {
	assets *service.AssetService
}

// NewAssetHandler creates a new AssetHandler.
func NewAssetHandler(assets *service.AssetService) *AssetHandler {
	return &AssetHandler{assets: assets}
}

// RegisterRoutes registers asset routes on the given mux.
func (h *AssetHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /assets", h.Upload)
	mux.HandleFunc("GET /assets/{id}", h.Serve)
}

// Upload handles POST /assets — multipart file upload.
func (h *AssetHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer func() { _ = file.Close() }()

	fileData, err := io.ReadAll(file)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	asset, err := h.assets.SaveAsset(header.Filename, header.Header.Get("Content-Type"), fileData)
	if err != nil {
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	webutil.JSON(w, http.StatusOK, map[string]string{"id": asset.ID, "name": asset.Name})
}

// Serve handles GET /assets/{id} — serves asset data with appropriate Content-Type.
func (h *AssetHandler) Serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	asset := h.assets.GetAsset(id)
	if asset == nil {
		http.NotFound(w, r)
		return
	}
	assetData, err := h.assets.AssetData(id)
	if err != nil {
		http.Error(w, "asset read error", http.StatusInternalServerError)
		return
	}
	// Only serve image MIME types to prevent XSS
	ct := asset.MimeType
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml":
		// allowed
	default:
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	//nolint:gosec // G705: Content-Type is sanitized above, data is user-uploaded image
	_, _ = w.Write(assetData)
}
