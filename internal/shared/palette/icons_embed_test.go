package palette

import "testing"

func TestSVG(t *testing.T) {
	data, err := SVG("wrench")
	if err != nil {
		t.Fatalf("SVG(wrench) error: %v", err)
	}
	if len(data) == 0 {
		t.Error("SVG(wrench) returned empty data")
	}
	if string(data[:4]) != "<svg" && string(data[:5]) != "<?xml" {
		t.Errorf("SVG(wrench) does not start with <svg: %s", string(data[:20]))
	}
}

func TestSVGNotFound(t *testing.T) {
	_, err := SVG("nonexistent")
	if err == nil {
		t.Error("SVG(nonexistent) should return error")
	}
}

func TestAllIconsHaveSVG(t *testing.T) {
	for _, ic := range icons {
		data, err := SVG(ic.Name)
		if err != nil {
			t.Errorf("SVG(%s) error: %v", ic.Name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("SVG(%s) returned empty data", ic.Name)
		}
	}
}
