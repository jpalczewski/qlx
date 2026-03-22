package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// I18nHandler handles HTTP requests for internationalization data.
type I18nHandler struct {
	translations *webutil.Translations
}

// NewI18nHandler creates a new I18nHandler.
func NewI18nHandler(translations *webutil.Translations) *I18nHandler {
	return &I18nHandler{translations: translations}
}

// RegisterRoutes registers i18n routes on the given mux.
func (h *I18nHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /i18n/{lang}", h.Translations)
}

// Translations handles GET /i18n/{lang} — returns merged translation map as JSON.
func (h *I18nHandler) Translations(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	merged := h.translations.Merged(lang)
	webutil.JSON(w, http.StatusOK, merged)
}
