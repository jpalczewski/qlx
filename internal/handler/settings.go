package handler

import (
	"net/http"
)

// SettingsHandler handles HTTP requests for application settings.
type SettingsHandler struct {
	resp Responder
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(resp Responder) *SettingsHandler {
	return &SettingsHandler{resp: resp}
}

// RegisterRoutes registers settings routes on the given mux.
func (h *SettingsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings", h.Settings)
	mux.HandleFunc("POST /set-lang", h.SetLang)
}

// Settings handles GET /settings — renders the settings page.
func (h *SettingsHandler) Settings(w http.ResponseWriter, r *http.Request) {
	h.resp.Respond(w, r, http.StatusOK, nil, "settings", func() any {
		return nil
	})
}

// SetLang handles POST /set-lang — sets the language cookie and redirects back.
func (h *SettingsHandler) SetLang(w http.ResponseWriter, r *http.Request) {
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
		referer = "/"
	}
	h.resp.Redirect(w, r, referer, map[string]any{"ok": true, "lang": lang})
}
