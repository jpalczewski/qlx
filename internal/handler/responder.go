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

	// RenderPartial renders a named define block directly without a layout wrapper.
	// Returns true if the partial was rendered, false if not supported (e.g. JSON responder).
	RenderPartial(w http.ResponseWriter, r *http.Request, tmpl, define string, data any) bool
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

func (j *JSONResponder) RenderPartial(_ http.ResponseWriter, _ *http.Request, _, _ string, _ any) bool {
	return false
}
