package label

import "testing"

func TestRenderPages_FirstPageOnly(t *testing.T) {
	media := MediaInfo{WidthPx: 384, HeightPx: 160, DPI: 203}
	data := LabelData{Name: "Synthetic", Location: "Room", QRContent: "/items/x"}

	all, err := RenderPages(data, PairTemplate, media, RenderOpts{})
	if err != nil || len(all) != 2 {
		t.Fatalf("pair pages = %d, err %v, want 2", len(all), err)
	}
	first, err := RenderPages(data, PairTemplate, media, RenderOpts{FirstPageOnly: true})
	if err != nil || len(first) != 1 {
		t.Fatalf("first-only pages = %d, err %v, want 1", len(first), err)
	}
	if first[0].Bounds() != all[0].Bounds() {
		t.Errorf("first-only page bounds = %v, want %v", first[0].Bounds(), all[0].Bounds())
	}
	single, err := RenderPages(data, "simple", media, RenderOpts{FirstPageOnly: true})
	if err != nil || len(single) != 1 {
		t.Errorf("schema template pages = %d, err %v, want 1", len(single), err)
	}
}
