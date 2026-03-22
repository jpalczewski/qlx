package handler

import (
	"html/template"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

// HTMLResponder negotiates response format based on request headers.
// HX-Request -> HTML partial, Accept: application/json -> JSON, otherwise -> full HTML page.
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

// RenderPartial renders a named define block directly (no layout).
// Use this for HTMX partial responses (fragments, not full pages).
func (h *HTMLResponder) RenderPartial(w http.ResponseWriter, r *http.Request, tmplName, defineName string, data any) {
	t, ok := h.templates[tmplName]
	if !ok {
		http.Error(w, "template not found: "+tmplName, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := PageData{
		Lang:       langFromRequest(r),
		translator: h.translations,
		Data:       data,
	}
	if err := t.ExecuteTemplate(w, defineName, page); err != nil {
		webutil.LogError("template execute: %v", err)
	}
}

func langFromRequest(r *http.Request) string {
	if v := r.Context().Value(webutil.LangKey); v != nil {
		return v.(string)
	}
	return "pl"
}
