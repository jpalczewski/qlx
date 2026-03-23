package service

import "github.com/erxyi/qlx/internal/store"

// TemplateService manages label template operations.
type TemplateService struct {
	store store.TemplateStore
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(s store.TemplateStore) *TemplateService {
	return &TemplateService{store: s}
}

func (s *TemplateService) AllTemplates() []store.Template {
	return s.store.AllTemplates()
}

func (s *TemplateService) GetTemplate(id string) *store.Template {
	return s.store.GetTemplate(id)
}

func (s *TemplateService) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	return s.store.CreateTemplate(name, tags, target, widthMM, heightMM, widthPx, heightPx, elements)
}

func (s *TemplateService) UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	return s.store.UpdateTemplate(id, name, tags, target, widthMM, heightMM, widthPx, heightPx, elements)
}

func (s *TemplateService) DeleteTemplate(id string) error {
	return s.store.DeleteTemplate(id)
}
