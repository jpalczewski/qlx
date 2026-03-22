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
	img, err := renderSchema(schema, data, 384)
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
	img, err := renderSchema(schema, data, 384)
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
	img, err := renderSchema(schema, data, 384)
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
	img, err := renderSchema(schema, data, 200)
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
	img, err := renderSchema(schema, data, 384)
	if err != nil {
		t.Fatalf("renderSchema: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}
