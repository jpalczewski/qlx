package handler

import "github.com/erxyi/qlx/internal/shared/webutil"

// PageData is the top-level template context for all page renders.
type PageData struct {
	Lang       string
	translator *webutil.Translations
	Data       any
}

// T returns the translation for key in the active language.
func (p PageData) T(key string) string {
	return p.translator.Get(p.Lang, key)
}
