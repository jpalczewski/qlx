package service

import "github.com/erxyi/qlx/internal/store"

// TemplateService manages label template operations.
type TemplateService struct {
	store interface {
		TemplateStore
		Saveable
	}
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(s interface {
	TemplateStore
	Saveable
}) *TemplateService {
	return &TemplateService{store: s}
}

func (s *TemplateService) AllTemplates() []store.Template {
	return s.store.AllTemplates()
}

func (s *TemplateService) GetTemplate(id string) *store.Template {
	return s.store.GetTemplate(id)
}

func (s *TemplateService) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*store.Template, error) {
	t := s.store.CreateTemplate(name, tags, target, widthMM, heightMM, widthPx, heightPx, elements)
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TemplateService) SaveTemplate(t store.Template) error {
	s.store.SaveTemplate(t)
	return s.store.Save()
}

func (s *TemplateService) DeleteTemplate(id string) error {
	s.store.DeleteTemplate(id)
	return s.store.Save()
}
