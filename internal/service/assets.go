package service

import "github.com/erxyi/qlx/internal/store"

// AssetService manages asset (image) operations.
type AssetService struct {
	store interface {
		AssetStore
		Saveable
	}
}

// NewAssetService creates a new AssetService.
func NewAssetService(s interface {
	AssetStore
	Saveable
}) *AssetService {
	return &AssetService{store: s}
}

func (s *AssetService) SaveAsset(name, mimeType string, data []byte) (*store.Asset, error) {
	asset, err := s.store.SaveAsset(name, mimeType, data)
	if err != nil {
		return nil, err
	}
	if err := s.store.Save(); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *AssetService) GetAsset(id string) *store.Asset {
	return s.store.GetAsset(id)
}

func (s *AssetService) AssetData(id string) ([]byte, error) {
	return s.store.AssetData(id)
}
