// Package validate provides input validation for entity names and descriptions.
package validate

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	// MaxNameLength is the maximum allowed length for container and item names.
	MaxNameLength = 100
	// MaxDescriptionLength is the maximum allowed length for descriptions.
	MaxDescriptionLength = 500
	// MaxTagNameLength is the maximum allowed length for tag names.
	MaxTagNameLength = 50
)

var (
	// ErrNameRequired indicates a required name field was empty.
	ErrNameRequired = errors.New("name is required")
	// ErrNameTooLong indicates the name exceeds the maximum length.
	ErrNameTooLong = errors.New("name exceeds maximum length")
	// ErrDescriptionTooLong indicates the description exceeds the maximum length.
	ErrDescriptionTooLong = errors.New("description exceeds maximum length")
	// ErrInvalidCharacters indicates the text contains control characters.
	ErrInvalidCharacters = errors.New("contains invalid characters")
)

// Name validates a required name field: must be non-empty after trimming,
// within maxLen characters, and free of control characters (tabs allowed).
func Name(name string, maxLen int) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrNameRequired
	}
	if len(trimmed) > maxLen {
		return fmt.Errorf("%w: %d (max %d)", ErrNameTooLong, len(trimmed), maxLen)
	}
	if containsControlChars(trimmed) {
		return ErrInvalidCharacters
	}
	return nil
}

// OptionalText validates an optional text field: max length and no control characters.
// Empty strings are valid.
func OptionalText(text string, maxLen int) error {
	if text == "" {
		return nil
	}
	if len(text) > maxLen {
		return fmt.Errorf("%w: %d (max %d)", ErrDescriptionTooLong, len(text), maxLen)
	}
	if containsControlChars(text) {
		return ErrInvalidCharacters
	}
	return nil
}

// containsControlChars returns true if s contains any Unicode control character
// other than tab (\t) and newline (\n).
func containsControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return true
		}
	}
	return false
}
