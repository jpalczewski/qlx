package webutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestTranslations_Get_Fallback(t *testing.T) {
	tr := NewTranslations()
	tr.langs["en"] = map[string]string{"hello": "Hello"}
	tr.langs["pl"] = map[string]string{"hello": "Cześć"}

	if got := tr.Get("pl", "hello"); got != "Cześć" {
		t.Errorf("Get(pl, hello) = %q, want Cześć", got)
	}
	if got := tr.Get("pl", "missing"); got != "missing" {
		t.Errorf("Get(pl, missing) = %q, want missing (key fallback)", got)
	}
	if got := tr.Get("de", "hello"); got != "Hello" {
		t.Errorf("Get(de, hello) = %q, want Hello (en fallback)", got)
	}
}

func TestTranslations_LoadFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"i18n/en/nav.json": {Data: []byte(`{"nav.home":"Home"}`)},
		"i18n/pl/nav.json": {Data: []byte(`{"nav.home":"Magazyn"}`)},
	}
	tr := NewTranslations()
	if err := tr.LoadFromFS(fsys, "i18n"); err != nil {
		t.Fatal(err)
	}
	if got := tr.Get("en", "nav.home"); got != "Home" {
		t.Errorf("Get(en) = %q, want Home", got)
	}
	if got := tr.Get("pl", "nav.home"); got != "Magazyn" {
		t.Errorf("Get(pl) = %q, want Magazyn", got)
	}
}

func TestLangMiddleware_Cookie(t *testing.T) {
	handler := LangMiddleware("pl")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Context().Value(LangKey).(string)
		_, _ = w.Write([]byte(lang))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "en"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Body.String() != "en" {
		t.Errorf("got %q, want en", rec.Body.String())
	}
}

func TestLangMiddleware_Default(t *testing.T) {
	handler := LangMiddleware("pl")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Context().Value(LangKey).(string)
		_, _ = w.Write([]byte(lang))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Body.String() != "pl" {
		t.Errorf("got %q, want pl", rec.Body.String())
	}
}
