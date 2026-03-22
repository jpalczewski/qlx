package label

import "testing"

func TestRenderSimple(t *testing.T) {
	data := LabelData{Name: "Test Item"}
	img, err := Render(data, "simple", 384, 203, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() == 0 {
		t.Error("height should be > 0")
	}
}

func TestRenderStandard(t *testing.T) {
	data := LabelData{
		Name:      "HDMI Cable",
		Location:  "Room → Shelf",
		QRContent: "https://qlx.local/item/123",
	}
	img, err := Render(data, "standard", 696, 300, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 696 {
		t.Errorf("width = %d, want 696", img.Bounds().Dx())
	}
}

func TestRenderDetailed(t *testing.T) {
	data := LabelData{
		Name:        "HDMI Cable 2m",
		Description: "High speed 4K",
		Location:    "Room → Shelf → Box",
		QRContent:   "https://qlx.local/item/123",
		BarcodeID:   "abc-123-def",
	}
	img, err := Render(data, "detailed", 696, 300, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 696 {
		t.Errorf("width = %d, want 696", img.Bounds().Dx())
	}
}

func TestRenderCompact(t *testing.T) {
	data := LabelData{Name: "Cable", Description: "HDMI 2m"}
	img, err := Render(data, "compact", 384, 203, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, err := Render(LabelData{Name: "X"}, "nonexistent", 384, 203, RenderOpts{})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestRender_PrintDate(t *testing.T) {
	data := LabelData{Name: "Test"}
	imgWithout, err := Render(data, "simple", 384, 203, RenderOpts{})
	if err != nil {
		t.Fatalf("Render without date: %v", err)
	}
	imgWith, err := Render(data, "simple", 384, 203, RenderOpts{PrintDate: true})
	if err != nil {
		t.Fatalf("Render with date: %v", err)
	}
	if imgWith.Bounds().Dy() <= imgWithout.Bounds().Dy() {
		t.Errorf("expected taller with date: %d vs %d",
			imgWith.Bounds().Dy(), imgWithout.Bounds().Dy())
	}
}
