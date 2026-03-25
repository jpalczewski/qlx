package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// ExportService handles data export operations.
type ExportService struct {
	store store.ExportStore
}

// NewExportService creates a new ExportService.
func NewExportService(s store.ExportStore) *ExportService {
	return &ExportService{store: s}
}

// ExportCSV writes CSV export to w.
func (s *ExportService) ExportCSV(w io.Writer, containerID string, recursive bool) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}
	paths := buildContainerPaths(containers)
	return FormatCSV(w, items, paths)
}

// ExportJSON writes JSON export to w.
func (s *ExportService) ExportJSON(w io.Writer, containerID string, recursive bool) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}

	grouped := (containerID != "" && recursive) || containerID == ""
	if grouped {
		return FormatJSONGrouped(w, containers, items)
	}
	paths := buildContainerPaths(containers)
	return FormatJSONFlat(w, items, paths)
}

// ExportMarkdown writes Markdown export to w.
func (s *ExportService) ExportMarkdown(w io.Writer, containerID string, recursive bool, style string) error {
	items, containers, err := s.fetchExportData(containerID, recursive)
	if err != nil {
		return err
	}
	paths := buildContainerPaths(containers)

	switch style {
	case "document":
		return FormatMarkdownDocument(w, containers, items)
	case "both":
		if err := FormatMarkdownTable(w, items, paths); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "---"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		return FormatMarkdownDocument(w, containers, items)
	default: // "table"
		return FormatMarkdownTable(w, items, paths)
	}
}

// ExportToString is a convenience wrapper that returns the formatted export as a string.
func (s *ExportService) ExportToString(format, containerID string, recursive bool, mdStyle string) (string, error) {
	var buf bytes.Buffer
	var err error

	switch format {
	case "csv":
		err = s.ExportCSV(&buf, containerID, recursive)
	case "json":
		err = s.ExportJSON(&buf, containerID, recursive)
	case "md":
		err = s.ExportMarkdown(&buf, containerID, recursive, mdStyle)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// fetchExportData loads items and containers for the given scope.
func (s *ExportService) fetchExportData(containerID string, recursive bool) ([]store.ExportItem, []store.Container, error) {
	if containerID == "" {
		items, err := s.store.ExportItems("", false)
		if err != nil {
			return nil, nil, err
		}
		containers := s.store.AllContainers()
		return items, containers, nil
	}

	items, err := s.store.ExportItems(containerID, recursive)
	if err != nil {
		return nil, nil, err
	}

	var containers []store.Container
	if recursive {
		containers, err = s.store.ExportContainerTree(containerID)
		if err != nil {
			return nil, nil, err
		}
	} else {
		containers = s.store.AllContainers()
	}
	return items, containers, nil
}

// SanitizeFilename returns a filesystem-safe version of a container name.
func SanitizeFilename(name string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	return strings.ToLower(r.Replace(name))
}
