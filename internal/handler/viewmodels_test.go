package handler

import (
	"testing"

	"github.com/erxyi/qlx/internal/embedded"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

func TestPageDataPlural(t *testing.T) {
	tr := webutil.NewTranslations()
	if err := tr.LoadFromFS(embedded.Static, "static/i18n"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		lang string
		n    int
		want string
	}{
		{"pl", 0, "0 przedmiotów"},
		{"pl", 1, "1 przedmiot"},
		{"pl", 2, "2 przedmioty"},
		{"pl", 4, "4 przedmioty"},
		{"pl", 5, "5 przedmiotów"},
		{"pl", 12, "12 przedmiotów"},
		{"pl", 22, "22 przedmioty"},
		{"pl", 112, "112 przedmiotów"},
		{"en", 1, "1 item"},
		{"en", 3, "3 items"},
	}
	for _, tt := range tests {
		p := PageData{Lang: tt.lang, translator: tr}
		if got := p.Plural("home.count", tt.n); got != tt.want {
			t.Errorf("%s %d = %q, want %q", tt.lang, tt.n, got, tt.want)
		}
	}
}
