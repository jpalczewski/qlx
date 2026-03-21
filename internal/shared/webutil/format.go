package webutil

import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// FormatContainerPath joins container names with the given separator.
func FormatContainerPath(path []store.Container, sep string) string {
	names := make([]string, len(path))
	for i, c := range path {
		names[i] = c.Name
	}
	return strings.Join(names, sep)
}
