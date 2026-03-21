package webutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// LangContextKey is the type used for the language context key to avoid collisions.
type LangContextKey string

// LangKey is the context key under which the resolved language tag is stored.
const LangKey LangContextKey = "lang"

// LangMiddleware resolves the active language for each request.
// Priority: lang cookie > Accept-Language header > defaultLang.
func LangMiddleware(defaultLang string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lang := defaultLang
			if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
				lang = c.Value
			} else {
				if parsed := parseAcceptLanguage(r); parsed != "" {
					lang = parsed
				}
			}
			ctx := context.WithValue(r.Context(), LangKey, lang)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseAcceptLanguage extracts the first two-character language tag from the Accept-Language header.
func parseAcceptLanguage(r *http.Request) string {
	header := r.Header.Get("Accept-Language")
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if len(tag) >= 2 {
			return tag[:2]
		}
	}
	return ""
}

// Translations is an i18n registry that supports multiple languages with English fallback.
type Translations struct {
	langs map[string]map[string]string
}

// NewTranslations returns an empty Translations registry.
func NewTranslations() *Translations {
	return &Translations{langs: make(map[string]map[string]string)}
}

// Get returns the translation for key in lang, falling back to "en", then the key itself.
func (t *Translations) Get(lang, key string) string {
	if val, ok := t.langs[lang][key]; ok {
		return val
	}
	if val, ok := t.langs["en"][key]; ok {
		return val
	}
	return key
}

// Merged returns a flat map of all translations for lang, with English as the base fallback.
func (t *Translations) Merged(lang string) map[string]string {
	merged := make(map[string]string)
	for k, v := range t.langs["en"] {
		merged[k] = v
	}
	for k, v := range t.langs[lang] {
		merged[k] = v
	}
	return merged
}

// LoadFromFS walks fsys from root, loading all JSON files found under i18n/<lang>/ directories.
// Each JSON file must be a flat object mapping string keys to string values.
// Keys from multiple files in the same language are merged together.
func (t *Translations) LoadFromFS(fsys fs.FS, root string) error {
	return fs.WalkDir(fsys, root, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(fpath, ".json") {
			return err
		}
		parts := strings.Split(fpath, "/")
		var lang string
		for i, p := range parts {
			if p == "i18n" && i+1 < len(parts) {
				lang = parts[i+1]
				break
			}
		}
		if lang == "" {
			return nil
		}
		data, err := fs.ReadFile(fsys, fpath)
		if err != nil {
			return err
		}
		var entries map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("parse %s: %w", fpath, err)
		}
		if t.langs[lang] == nil {
			t.langs[lang] = make(map[string]string)
		}
		for k, v := range entries {
			t.langs[lang][k] = v
		}
		return nil
	})
}
