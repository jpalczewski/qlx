package service

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

func TestTemplateService_CreateTemplate(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewTemplateService(s)

	tmpl, err := svc.CreateTemplate("Test", nil, "ql700", 62, 29, 720, 320, "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Name != "Test" {
		t.Fatalf("expected Test, got %s", tmpl.Name)
	}

	all := svc.AllTemplates()
	if len(all) != 1 {
		t.Fatalf("expected 1 template, got %d", len(all))
	}
}

func TestTemplateService_DeleteTemplate(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewTemplateService(s)

	tmpl, _ := svc.CreateTemplate("ToDelete", nil, "ql700", 62, 29, 720, 320, "{}")
	if err := svc.DeleteTemplate(tmpl.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.AllTemplates()) != 0 {
		t.Fatal("expected 0 templates after delete")
	}
}
