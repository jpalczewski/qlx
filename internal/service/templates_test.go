package service

import (
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

type mockTemplateStore struct {
	templates map[string]*store.Template
}

func newMockTemplateStore() *mockTemplateStore {
	return &mockTemplateStore{templates: make(map[string]*store.Template)}
}

func (m *mockTemplateStore) AllTemplates() []store.Template {
	var result []store.Template
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result
}

func (m *mockTemplateStore) GetTemplate(id string) *store.Template {
	t, ok := m.templates[id]
	if !ok {
		return nil
	}
	return t
}

func (m *mockTemplateStore) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	t := &store.Template{
		ID:        "tmpl-1",
		Name:      name,
		Tags:      tags,
		Target:    target,
		WidthMM:   widthMM,
		HeightMM:  heightMM,
		WidthPx:   widthPx,
		HeightPx:  heightPx,
		Elements:  elements,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.templates[t.ID] = t
	return t, nil
}

func (m *mockTemplateStore) UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, store.ErrTemplateNotFound
	}
	t.Name = name
	t.Tags = tags
	t.Target = target
	t.WidthMM = widthMM
	t.HeightMM = heightMM
	t.WidthPx = widthPx
	t.HeightPx = heightPx
	t.Elements = elements
	t.UpdatedAt = time.Now()
	return t, nil
}

func (m *mockTemplateStore) DeleteTemplate(id string) error {
	if _, ok := m.templates[id]; !ok {
		return store.ErrTemplateNotFound
	}
	delete(m.templates, id)
	return nil
}

func TestTemplateService_CreateTemplate(t *testing.T) {
	s := newMockTemplateStore()
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
	s := newMockTemplateStore()
	svc := NewTemplateService(s)

	tmpl, _ := svc.CreateTemplate("ToDelete", nil, "ql700", 62, 29, 720, 320, "{}")
	if err := svc.DeleteTemplate(tmpl.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.AllTemplates()) != 0 {
		t.Fatal("expected 0 templates after delete")
	}
}
