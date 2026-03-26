package service

import (
	"fmt"

	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

// NoteService handles note CRUD operations.
type NoteService struct {
	store interface {
		store.NoteStore
		store.ContainerStore
		store.ItemStore
	}
}

// NewNoteService creates a new NoteService backed by the given store.
func NewNoteService(s interface {
	store.NoteStore
	store.ContainerStore
	store.ItemStore
}) *NoteService {
	return &NoteService{store: s}
}

// GetNote returns the note with the given ID, or nil.
func (s *NoteService) GetNote(id string) *store.Note {
	return s.store.GetNote(id)
}

// CreateNote validates and creates a new note.
func (s *NoteService) CreateNote(containerID, itemID, title, content, color, icon string) (*store.Note, error) {
	if err := validate.Name(title, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(content, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	if (containerID == "") == (itemID == "") {
		return nil, fmt.Errorf("exactly one of container_id or item_id must be set")
	}
	if containerID != "" {
		if s.store.GetContainer(containerID) == nil {
			return nil, store.ErrContainerNotFound
		}
	}
	if itemID != "" {
		if s.store.GetItem(itemID) == nil {
			return nil, store.ErrItemNotFound
		}
	}
	note := s.store.CreateNote(containerID, itemID, title, content, color, icon)
	return note, nil
}

// UpdateNote validates and updates a note.
func (s *NoteService) UpdateNote(id, title, content, color, icon string) (*store.Note, error) {
	if err := validate.Name(title, validate.MaxNameLength); err != nil {
		return nil, err
	}
	if err := validate.OptionalText(content, validate.MaxDescriptionLength); err != nil {
		return nil, err
	}
	if color != "" && !palette.ValidColor(color) {
		return nil, fmt.Errorf("invalid color: %s", color)
	}
	if icon != "" && !palette.ValidIcon(icon) {
		return nil, fmt.Errorf("invalid icon: %s", icon)
	}
	return s.store.UpdateNote(id, title, content, color, icon)
}

// DeleteNote deletes a note.
func (s *NoteService) DeleteNote(id string) error {
	return s.store.DeleteNote(id)
}

// ContainerNotes returns all notes for a container.
func (s *NoteService) ContainerNotes(containerID string) []store.Note {
	return s.store.ContainerNotes(containerID)
}

// ItemNotes returns all notes for an item.
func (s *NoteService) ItemNotes(itemID string) []store.Note {
	return s.store.ItemNotes(itemID)
}
