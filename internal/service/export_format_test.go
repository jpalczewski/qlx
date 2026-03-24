package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

var testTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

func testContainers() []store.Container {
	return []store.Container{
		{ID: "c1", Name: "Root", ParentID: ""},
		{ID: "c2", Name: "Child", ParentID: "c1"},
	}
}

func testItems() []store.ExportItem {
	return []store.ExportItem{
		{ID: "i1", Name: "Widget", Description: "A widget", Quantity: 3, ContainerID: "c1", TagNames: []string{"Electronics", "Fragile"}, CreatedAt: testTime},
		{ID: "i2", Name: "Gadget", Description: "", Quantity: 1, ContainerID: "c2", TagNames: nil, CreatedAt: testTime},
	}
}

func TestFormatCSV(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())
	err := FormatCSV(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "item_id") {
		t.Error("header missing item_id")
	}
	if !strings.Contains(lines[1], "Electronics;Fragile") {
		t.Error("row 1 missing tags")
	}
	if !strings.Contains(lines[1], "Root") {
		t.Error("row 1 missing path")
	}
	if !strings.Contains(lines[2], "Root > Child") {
		t.Error("row 2 missing nested path")
	}
}

func TestFormatJSONFlat(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())
	err := FormatJSONFlat(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"container_path"`) {
		t.Error("missing container_path")
	}
	if !strings.Contains(buf.String(), `"Widget"`) {
		t.Error("missing item name")
	}
}

func TestFormatJSONGrouped(t *testing.T) {
	var buf bytes.Buffer
	err := FormatJSONGrouped(&buf, testContainers(), testItems())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"children"`) {
		t.Error("missing children")
	}
	if !strings.Contains(buf.String(), `"Root"`) {
		t.Error("missing root container")
	}
}

func TestFormatMarkdownTable(t *testing.T) {
	var buf bytes.Buffer
	paths := buildContainerPaths(testContainers())
	err := FormatMarkdownTable(&buf, testItems(), paths)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "| item_id") {
		t.Error("missing table header")
	}
	if !strings.Contains(out, "| ---") {
		t.Error("missing separator")
	}
	if !strings.Contains(out, "Widget") {
		t.Error("missing item data")
	}
}

func TestFormatMarkdownDocument(t *testing.T) {
	var buf bytes.Buffer
	err := FormatMarkdownDocument(&buf, testContainers(), testItems())
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "## Root") {
		t.Error("missing Root header")
	}
	if !strings.Contains(out, "## Child") {
		t.Error("missing Child header")
	}
	if !strings.Contains(out, "- **Widget**") {
		t.Error("missing item bullet")
	}
}

func TestFormatCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatCSV(&buf, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("got %d lines, want 1 (header only)", len(lines))
	}
}
