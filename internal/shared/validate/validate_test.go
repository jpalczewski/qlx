package validate

import (
	"errors"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		wantErr error
	}{
		{"valid short name", "Box A", 100, nil},
		{"empty", "", 100, ErrNameRequired},
		{"whitespace only", "   ", 100, ErrNameRequired},
		{"tab only", "\t", 100, ErrNameRequired},
		{"exactly at limit", strings.Repeat("a", 100), 100, nil},
		{"one over limit", strings.Repeat("a", 101), 100, ErrNameTooLong},
		{"way over limit", strings.Repeat("x", 200), 50, ErrNameTooLong},
		{"contains null byte", "hello\x00world", 100, ErrInvalidCharacters},
		{"contains carriage return", "hello\rworld", 100, ErrInvalidCharacters},
		{"tab allowed", "hello\tworld", 100, nil},
		{"newline allowed in name", "hello\nworld", 100, nil},
		{"unicode name", "Półka Główna", 100, nil},
		{"emoji name", "📦 Box", 100, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Name(tt.input, tt.maxLen)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Name(%q, %d) = %v, want nil", tt.input, tt.maxLen, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Name(%q, %d) = %v, want %v", tt.input, tt.maxLen, err, tt.wantErr)
			}
		})
	}
}

func TestOptionalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		wantErr error
	}{
		{"empty is valid", "", 500, nil},
		{"valid text", "A nice description", 500, nil},
		{"exactly at limit", strings.Repeat("b", 500), 500, nil},
		{"one over limit", strings.Repeat("b", 501), 500, ErrDescriptionTooLong},
		{"contains null byte", "desc\x00ription", 500, ErrInvalidCharacters},
		{"tab allowed", "desc\twith\ttabs", 500, nil},
		{"newline allowed", "line1\nline2", 500, nil},
		{"unicode text", "Opis z polskimi znakami: ąćęłńóśźż", 500, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := OptionalText(tt.input, tt.maxLen)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("OptionalText(%q, %d) = %v, want nil", tt.input, tt.maxLen, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("OptionalText(%q, %d) = %v, want %v", tt.input, tt.maxLen, err, tt.wantErr)
			}
		})
	}
}
