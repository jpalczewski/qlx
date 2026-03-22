package label

import "testing"

func TestRenderSchema(t *testing.T) {
	schema := Schema{
		Name:    "test",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 20, Align: "center", Color: "#000000"},
			{Slot: "description", FontSize: 10, Align: "center", Color: "#3c3c3c"},
		},
	}
	data := LabelData{Name: "Test Item", Description: "A test description"}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestRenderSchemaWithQR(t *testing.T) {
	schema := Schema{
		Name:    "with-qr",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 13, Align: "left", Color: "#000000"},
			{Slot: "location", FontSize: 13, Align: "left", Color: "#505050"},
			{Slot: "qr", Size: 80},
		},
	}
	data := LabelData{
		Name:      "Cable",
		Location:  "Pokój → Półka → Pudełko",
		QRContent: "https://qlx.local/item/123",
	}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderSchemaWithBarcode(t *testing.T) {
	schema := Schema{
		Name:    "with-barcode",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 13, Align: "left", Color: "#000000"},
			{Slot: "barcode", Height: 32},
		},
	}
	data := LabelData{Name: "Item", BarcodeID: "abc-123"}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestRenderSchemaTextWrapping(t *testing.T) {
	schema := Schema{
		Name:    "wrap-test",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 20, Align: "center", Wrap: true, Color: "#000000"},
		},
	}
	data := LabelData{Name: "This is a very long title that should definitely wrap to multiple lines on a narrow label"}
	img, err := renderSchema(schema, data, 200, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	// With wrapping, height should be taller than a single line
	singleLineH := 8*2 + 24 // padding*2 + approx line height at 20px
	if img.Bounds().Dy() <= singleLineH {
		t.Errorf("height = %d, expected taller due to wrapping", img.Bounds().Dy())
	}
}

func TestRenderSchemaPolishChars(t *testing.T) {
	schema := Schema{
		Name:    "polish",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 13, Align: "left", Color: "#000000"},
			{Slot: "location", FontSize: 13, Align: "left", Color: "#505050"},
		},
	}
	data := LabelData{
		Name:     "Kątówka ośmiokątna",
		Location: "Łódź → Półka → Skrzynia żółta",
	}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderSchema_WithTags(t *testing.T) {
	schema := Schema{
		Name:    "tags-test",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 13, Align: "left", Color: "#000000"},
			{Slot: "tags", FontSize: 10, Align: "left", Color: "#505050", ShowPath: "auto"},
		},
	}
	data := LabelData{
		Name: "Test Item",
		Tags: []LabelTag{
			{Name: "arduino", Path: []string{"elektronika", "arduino"}},
		},
	}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestRenderSchema_WithTagsIgnoredByExistingSchemas(t *testing.T) {
	// Existing schemas have no tags slot, so tags data is simply ignored.
	data := LabelData{
		Name: "Test Item",
		Tags: []LabelTag{
			{Name: "arduino", Path: []string{"elektronika", "arduino"}},
		},
	}
	img, err := Render(data, "simple", 384, 203, RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderSchema_WithIcon(t *testing.T) {
	data := LabelData{Name: "My Item", Icon: "package"}
	img, err := Render(data, "simple", 384, 203, RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderSchema_WithChildren(t *testing.T) {
	schema := Schema{
		Name:    "children-test",
		Padding: 8,
		Elements: []Element{
			{Slot: "title", FontSize: 13, Align: "left", Color: "#000000"},
			{Slot: "children", FontSize: 10, Align: "left", Color: "#505050", Wrap: true},
		},
	}
	data := LabelData{
		Name: "Container",
		Children: []LabelChild{
			{Name: "Item 1"},
			{Name: "Item 2"},
			{Name: "Item 3"},
		},
	}
	img, err := renderSchema(schema, data, 384, RenderOpts{})
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestFormatTags_ShowPathTrue(t *testing.T) {
	tags := []LabelTag{
		{Name: "arduino", Path: []string{"elektronika", "arduino"}},
		{Name: "esp32", Path: []string{"elektronika", "esp32"}},
	}
	el := Element{ShowPath: "true"}
	got := formatTags(tags, el, nil, 0)
	want := "#elektronika>arduino #elektronika>esp32"
	if got != want {
		t.Errorf("formatTags(true) = %q, want %q", got, want)
	}
}

func TestFormatTags_ShowPathFalse(t *testing.T) {
	tags := []LabelTag{
		{Name: "arduino", Path: []string{"elektronika", "arduino"}},
		{Name: "esp32", Path: []string{"elektronika", "esp32"}},
	}
	el := Element{ShowPath: "false"}
	got := formatTags(tags, el, nil, 0)
	want := "#arduino #esp32"
	if got != want {
		t.Errorf("formatTags(false) = %q, want %q", got, want)
	}
}

func TestFormatTags_ShowPathAuto(t *testing.T) {
	tags := []LabelTag{
		{Name: "arduino", Path: []string{"elektronika", "arduino"}},
	}

	face, err := LoadFace("spleen", 13)
	if err != nil {
		t.Fatalf("LoadFace: %v", err)
	}

	// Wide enough for full path
	el := Element{ShowPath: "auto"}
	got := formatTags(tags, el, face, 500)
	want := "#elektronika>arduino"
	if got != want {
		t.Errorf("formatTags(auto, wide) = %q, want %q", got, want)
	}

	// Very narrow — should fall back to leaf only
	got = formatTags(tags, el, face, 10)
	want = "#arduino"
	if got != want {
		t.Errorf("formatTags(auto, narrow) = %q, want %q", got, want)
	}
}

func TestFormatTags_SingleSegmentPath(t *testing.T) {
	tags := []LabelTag{
		{Name: "misc", Path: []string{"misc"}},
	}
	el := Element{ShowPath: "true"}
	got := formatTags(tags, el, nil, 0)
	want := "#misc"
	if got != want {
		t.Errorf("formatTags(single segment) = %q, want %q", got, want)
	}
}

func TestFormatChildren(t *testing.T) {
	children := []LabelChild{
		{Name: "Arduino Uno"},
		{Name: "Breadboard"},
		{Name: "Jumper Wires"},
	}
	got := formatChildren(children)
	want := "Arduino Uno, Breadboard, Jumper Wires"
	if got != want {
		t.Errorf("formatChildren = %q, want %q", got, want)
	}
}

func TestFormatChildren_Empty(t *testing.T) {
	got := formatChildren(nil)
	if got != "" {
		t.Errorf("formatChildren(nil) = %q, want empty", got)
	}
}

func TestEffectiveFontFamily(t *testing.T) {
	tests := []struct {
		name     string
		el       Element
		schema   Schema
		expected string
	}{
		{"element override", Element{FontFamily: "terminus"}, Schema{FontFamily: "basic"}, "terminus"},
		{"schema default", Element{}, Schema{FontFamily: "terminus"}, "terminus"},
		{"fallback to spleen", Element{}, Schema{}, "spleen"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveFontFamily(tc.el, tc.schema)
			if got != tc.expected {
				t.Errorf("effectiveFontFamily = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestShowIcons(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		el       Element
		expected bool
	}{
		{"title default", Element{Slot: "title"}, true},
		{"tags default", Element{Slot: "tags"}, true},
		{"children default", Element{Slot: "children"}, true},
		{"description default", Element{Slot: "description"}, false},
		{"location default", Element{Slot: "location"}, false},
		{"explicit true", Element{Slot: "description", ShowIcons: boolPtr(true)}, true},
		{"explicit false", Element{Slot: "title", ShowIcons: boolPtr(false)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := showIcons(tc.el)
			if got != tc.expected {
				t.Errorf("showIcons = %v, want %v", got, tc.expected)
			}
		})
	}
}

// TestFormatTags_ShowPathTrue_NilFace verifies formatTags works with nil face for "true"/"false" modes.
func TestFormatTags_EmptyTags(t *testing.T) {
	el := Element{ShowPath: "true"}
	got := formatTags(nil, el, nil, 0)
	if got != "" {
		t.Errorf("formatTags(nil) = %q, want empty", got)
	}
}
