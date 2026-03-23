package service

import (
	"testing"

	"github.com/erxyi/qlx/internal/store"
)

type mockBulkStore struct {
	bulkMove   func(itemIDs, containerIDs []string, targetID string) []store.BulkError
	bulkDelete func(itemIDs, containerIDs []string) ([]string, []store.BulkError)
	bulkAddTag func(itemIDs, containerIDs []string, tagID string) error
}

func (m *mockBulkStore) BulkMove(itemIDs, containerIDs []string, targetID string) []store.BulkError {
	if m.bulkMove != nil {
		return m.bulkMove(itemIDs, containerIDs, targetID)
	}
	return nil
}
func (m *mockBulkStore) BulkDelete(itemIDs, containerIDs []string) ([]string, []store.BulkError) {
	if m.bulkDelete != nil {
		return m.bulkDelete(itemIDs, containerIDs)
	}
	return itemIDs, nil
}
func (m *mockBulkStore) BulkAddTag(itemIDs, containerIDs []string, tagID string) error {
	if m.bulkAddTag != nil {
		return m.bulkAddTag(itemIDs, containerIDs, tagID)
	}
	return nil
}

func TestBulkService_Move(t *testing.T) {
	tests := []struct {
		name      string
		mock      *mockBulkStore
		wantErr   bool
		wantBulkN int
	}{
		{
			name: "success no errors",
			mock: &mockBulkStore{},
		},
		{
			name: "with bulk errors",
			mock: &mockBulkStore{
				bulkMove: func(_, _ []string, _ string) []store.BulkError {
					return []store.BulkError{{ID: "c1", Reason: "not found"}}
				},
			},
			wantBulkN: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBulkService(tt.mock)
			errs, err := svc.Move([]string{"i1"}, []string{"c1"}, "target")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(errs) != tt.wantBulkN {
				t.Errorf("got %d bulk errors, want %d", len(errs), tt.wantBulkN)
			}
		})
	}
}

func TestBulkService_Delete(t *testing.T) {
	tests := []struct {
		name     string
		mock     *mockBulkStore
		wantErr  bool
		wantDelN int
	}{
		{
			name:     "success",
			mock:     &mockBulkStore{},
			wantDelN: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBulkService(tt.mock)
			deleted, _, err := svc.Delete([]string{"i1", "i2"}, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(deleted) != tt.wantDelN {
				t.Errorf("got %d deleted, want %d", len(deleted), tt.wantDelN)
			}
		})
	}
}

func TestBulkService_AddTag(t *testing.T) {
	tests := []struct {
		name    string
		mock    *mockBulkStore
		wantErr bool
	}{
		{
			name: "success",
			mock: &mockBulkStore{},
		},
		{
			name: "tag not found",
			mock: &mockBulkStore{
				bulkAddTag: func(_, _ []string, _ string) error {
					return store.ErrTagNotFound
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBulkService(tt.mock)
			err := svc.AddTag([]string{"i1"}, []string{"c1"}, "t1")
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
