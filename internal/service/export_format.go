package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

// buildContainerPaths walks parent pointers to build full path strings like "Root > Child > Grandchild".
func buildContainerPaths(containers []store.Container) map[string]string {
	byID := make(map[string]store.Container, len(containers))
	for _, c := range containers {
		byID[c.ID] = c
	}

	paths := make(map[string]string, len(containers))
	var resolve func(id string) string
	resolve = func(id string) string {
		if id == "" {
			return ""
		}
		if p, ok := paths[id]; ok {
			return p
		}
		c, ok := byID[id]
		if !ok {
			return id
		}
		if c.ParentID == "" {
			paths[id] = c.Name
		} else {
			paths[id] = resolve(c.ParentID) + " > " + c.Name
		}
		return paths[id]
	}
	for _, c := range containers {
		resolve(c.ID)
	}
	return paths
}

// jsonFlatItem is the structure used for flat JSON export with container_path resolved.
type jsonFlatItem struct {
	ID            string    `json:"item_id"`
	Name          string    `json:"item_name"`
	Quantity      int       `json:"quantity"`
	Tags          []string  `json:"tags"`
	Description   string    `json:"description"`
	ContainerPath string    `json:"container_path"`
	CreatedAt     time.Time `json:"created_at"`
}

// jsonContainerNode is a recursive node used in the grouped JSON export.
type jsonContainerNode struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Items    []store.ExportItem  `json:"items"`
	Children []jsonContainerNode `json:"children"`
}

// jsonGroupedExport is the top-level structure for grouped JSON export.
type jsonGroupedExport struct {
	Containers []jsonContainerNode `json:"containers"`
}

// FormatCSV writes a CSV export to w with header:
// item_id,item_name,quantity,tags,description,container_path,created_at
// Tags are joined with ";" (semicolon, no spaces).
func FormatCSV(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"item_id", "item_name", "quantity", "tags", "description", "container_path", "created_at"}); err != nil {
		return err
	}
	for _, item := range items {
		tags := strings.Join(item.TagNames, ";")
		path := ""
		if paths != nil {
			path = paths[item.ContainerID]
		}
		row := []string{
			item.ID,
			item.Name,
			fmt.Sprintf("%d", item.Quantity),
			tags,
			item.Description,
			path,
			item.CreatedAt.Format(time.RFC3339),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// FormatJSONFlat writes a JSON array of items to w with a resolved container_path field.
// Nil tag slices are rendered as empty arrays.
func FormatJSONFlat(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	out := make([]jsonFlatItem, len(items))
	for i, item := range items {
		tags := item.TagNames
		if tags == nil {
			tags = []string{}
		}
		path := ""
		if paths != nil {
			path = paths[item.ContainerID]
		}
		out[i] = jsonFlatItem{
			ID:            item.ID,
			Name:          item.Name,
			Quantity:      item.Quantity,
			Tags:          tags,
			Description:   item.Description,
			ContainerPath: path,
			CreatedAt:     item.CreatedAt,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// FormatJSONGrouped writes a nested JSON export to w, grouping items by container tree.
// Structure: {"containers": [{"id":..., "name":..., "items":[...], "children":[...]}]}
// Nil tag slices are rendered as empty arrays.
func FormatJSONGrouped(w io.Writer, containers []store.Container, items []store.ExportItem) error {
	// Index items by container ID.
	itemsByContainer := make(map[string][]store.ExportItem)
	for _, item := range items {
		tags := item.TagNames
		if tags == nil {
			tags = []string{}
			item.TagNames = tags
		}
		itemsByContainer[item.ContainerID] = append(itemsByContainer[item.ContainerID], item)
	}

	// Build a set of all container IDs.
	containerSet := make(map[string]bool, len(containers))
	for _, c := range containers {
		containerSet[c.ID] = true
	}

	// Index containers by parent.
	byParent := make(map[string][]store.Container)
	for _, c := range containers {
		byParent[c.ParentID] = append(byParent[c.ParentID], c)
	}

	// Find roots: containers whose parent is not in the set or is empty.
	var roots []store.Container
	for _, c := range containers {
		if c.ParentID == "" || !containerSet[c.ParentID] {
			roots = append(roots, c)
		}
	}

	var buildNode func(c store.Container) jsonContainerNode
	buildNode = func(c store.Container) jsonContainerNode {
		nodeItems := itemsByContainer[c.ID]
		if nodeItems == nil {
			nodeItems = []store.ExportItem{}
		}
		var children []jsonContainerNode
		for _, child := range byParent[c.ID] {
			children = append(children, buildNode(child))
		}
		if children == nil {
			children = []jsonContainerNode{}
		}
		return jsonContainerNode{
			ID:       c.ID,
			Name:     c.Name,
			Items:    nodeItems,
			Children: children,
		}
	}

	rootNodes := make([]jsonContainerNode, len(roots))
	for i, r := range roots {
		rootNodes[i] = buildNode(r)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonGroupedExport{Containers: rootNodes})
}

// FormatMarkdownTable writes a pipe-delimited Markdown table to w with columns:
// item_id, item_name, quantity, tags, description, container_path, created_at
// Tags are joined with "; " (semicolon-space for readability).
func FormatMarkdownTable(w io.Writer, items []store.ExportItem, paths map[string]string) error {
	header := "| item_id | item_name | quantity | tags | description | container_path | created_at |"
	sep := "| --- | --- | --- | --- | --- | --- | --- |"
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, sep); err != nil {
		return err
	}
	for _, item := range items {
		tags := strings.Join(item.TagNames, "; ")
		path := ""
		if paths != nil {
			path = paths[item.ContainerID]
		}
		row := fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %s |",
			item.ID,
			item.Name,
			item.Quantity,
			tags,
			item.Description,
			path,
			item.CreatedAt.Format(time.RFC3339),
		)
		if _, err := fmt.Fprintln(w, row); err != nil {
			return err
		}
	}
	return nil
}

// formatItemBullet formats a single export item as a Markdown bullet line.
func formatItemBullet(item store.ExportItem) string {
	var sb strings.Builder
	sb.WriteString("- **")
	sb.WriteString(item.Name)
	sb.WriteString("**")
	if item.Quantity > 1 {
		fmt.Fprintf(&sb, " (x%d)", item.Quantity)
	}
	if item.Description != "" {
		sb.WriteString(" — ")
		sb.WriteString(item.Description)
	}
	if len(item.TagNames) > 0 {
		sb.WriteString(" [")
		sb.WriteString(strings.Join(item.TagNames, ", "))
		sb.WriteString("]")
	}
	return sb.String()
}

// containerTree groups containers by parent and identifies roots within the given set.
func containerTree(containers []store.Container) (roots []store.Container, byParent map[string][]store.Container) {
	containerSet := make(map[string]bool, len(containers))
	for _, c := range containers {
		containerSet[c.ID] = true
	}
	byParent = make(map[string][]store.Container)
	for _, c := range containers {
		byParent[c.ParentID] = append(byParent[c.ParentID], c)
	}
	for _, c := range containers {
		if c.ParentID == "" || !containerSet[c.ParentID] {
			roots = append(roots, c)
		}
	}
	return roots, byParent
}

// FormatMarkdownDocument writes a hierarchical Markdown document to w, using container
// names as headers (## Name) and items as bullet points.
func FormatMarkdownDocument(w io.Writer, containers []store.Container, items []store.ExportItem) error {
	itemsByContainer := make(map[string][]store.ExportItem)
	for _, item := range items {
		itemsByContainer[item.ContainerID] = append(itemsByContainer[item.ContainerID], item)
	}

	roots, byParent := containerTree(containers)

	var writeNode func(c store.Container) error
	writeNode = func(c store.Container) error {
		if _, err := fmt.Fprintf(w, "## %s\n\n", c.Name); err != nil {
			return err
		}
		for _, item := range itemsByContainer[c.ID] {
			if _, err := fmt.Fprintln(w, formatItemBullet(item)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, ""); err != nil {
			return err
		}
		for _, child := range byParent[c.ID] {
			if err := writeNode(child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, r := range roots {
		if err := writeNode(r); err != nil {
			return err
		}
	}
	return nil
}
