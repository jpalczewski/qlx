package label

import (
	"sort"
	"testing"
)

func TestGetSchema(t *testing.T) {
	schema, ok := GetSchema("simple")
	if !ok {
		t.Fatal("GetSchema(simple) returned false")
	}
	if schema.Elements[0].Slot != "title" {
		t.Errorf("simple first slot = %q, want title", schema.Elements[0].Slot)
	}
	if schema.Elements[0].FontSize != 24 {
		t.Errorf("simple title font_size = %v, want 24", schema.Elements[0].FontSize)
	}

	_, ok = GetSchema("nonexistent")
	if ok {
		t.Error("GetSchema(nonexistent) should return false")
	}
}

func TestSchemaNames(t *testing.T) {
	names := SchemaNames()
	want := []string{"compact", "detailed", "micro", "simple", "standard"}
	if len(names) != len(want) {
		t.Fatalf("SchemaNames() = %v, want %v", names, want)
	}
	sort.Strings(names)
	for i, n := range names {
		if n != want[i] {
			t.Errorf("SchemaNames()[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestParseSchema(t *testing.T) {
	raw := `{
		"name": "simple",
		"padding": 8,
		"elements": [
			{"slot": "title", "font_size": 20, "align": "center", "wrap": true},
			{"slot": "description", "font_size": 10, "align": "center", "wrap": true}
		]
	}`
	schema, err := parseSchema([]byte(raw))
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if schema.Name != "simple" {
		t.Errorf("name = %q, want simple", schema.Name)
	}
	if schema.Padding != 8 {
		t.Errorf("padding = %d, want 8", schema.Padding)
	}
	if len(schema.Elements) != 2 {
		t.Fatalf("elements count = %d, want 2", len(schema.Elements))
	}
	if schema.Elements[0].Slot != "title" {
		t.Errorf("first slot = %q, want title", schema.Elements[0].Slot)
	}
	if schema.Elements[0].FontSize != 20 {
		t.Errorf("first font_size = %v, want 20", schema.Elements[0].FontSize)
	}
	if !schema.Elements[0].Wrap {
		t.Error("first wrap should be true")
	}
}

func TestParseSchemaDefaults(t *testing.T) {
	raw := `{
		"name": "minimal",
		"elements": [
			{"slot": "title"}
		]
	}`
	schema, err := parseSchema([]byte(raw))
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	el := schema.Elements[0]
	if el.FontSize != 13 {
		t.Errorf("default font_size = %v, want 13", el.FontSize)
	}
	if el.Align != "left" {
		t.Errorf("default align = %q, want left", el.Align)
	}
	if el.Color != "#000000" {
		t.Errorf("default color = %q, want #000000", el.Color)
	}
	if schema.Padding != 8 {
		t.Errorf("default padding = %d, want 8", schema.Padding)
	}
}
