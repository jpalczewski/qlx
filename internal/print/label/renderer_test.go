package label

import (
	"fmt"
	"testing"
)

func TestRenderSimple(t *testing.T) {
	data := LabelData{Name: "Test Item"}
	img, err := Render(data, "simple", MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}, RenderOpts{})
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
	img, err := Render(data, "standard", MediaInfo{WidthPx: 696, HeightPx: 0, DPI: 300}, RenderOpts{})
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
	img, err := Render(data, "detailed", MediaInfo{WidthPx: 696, HeightPx: 0, DPI: 300}, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 696 {
		t.Errorf("width = %d, want 696", img.Bounds().Dx())
	}
}

func TestRenderCompact(t *testing.T) {
	data := LabelData{Name: "Cable", Description: "HDMI 2m"}
	img, err := Render(data, "compact", MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, err := Render(LabelData{Name: "X"}, "nonexistent", MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}, RenderOpts{})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestRender_PrintDate(t *testing.T) {
	data := LabelData{Name: "Test"}
	imgWithout, err := Render(data, "simple", MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}, RenderOpts{})
	if err != nil {
		t.Fatalf("Render without date: %v", err)
	}
	imgWith, err := Render(data, "simple", MediaInfo{WidthPx: 384, HeightPx: 0, DPI: 203}, RenderOpts{PrintDate: true})
	if err != nil {
		t.Fatalf("Render with date: %v", err)
	}
	if imgWith.Bounds().Dy() <= imgWithout.Bounds().Dy() {
		t.Errorf("expected taller with date: %d vs %d",
			imgWith.Bounds().Dy(), imgWithout.Bounds().Dy())
	}
}

func TestRender_DieCut_FixedHeight(t *testing.T) {
	data := LabelData{Name: "Test Item", Description: "Desc", Location: "Room > Shelf"}
	media := MediaInfo{WidthPx: 384, HeightPx: 200, DPI: 203}
	img, err := Render(data, "standard", media, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width = %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() != 200 {
		t.Errorf("height = %d, want 200 (die-cut)", img.Bounds().Dy())
	}
}

func TestRender_DieCut_Overflow_ClipsHeight(t *testing.T) {
	tags := make([]LabelTag, 20)
	for i := range tags {
		tags[i] = LabelTag{Name: fmt.Sprintf("tag%d", i)}
	}
	data := LabelData{
		Name:     "Long Title That Might Wrap",
		Location: "Very Long Location Path > Sub > Sub > Sub",
		Tags:     tags,
	}
	media := MediaInfo{WidthPx: 384, HeightPx: 100, DPI: 203}
	img, err := Render(data, "detailed", media, RenderOpts{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if img.Bounds().Dy() != 100 {
		t.Errorf("height = %d, want 100 (die-cut clipped)", img.Bounds().Dy())
	}
}
