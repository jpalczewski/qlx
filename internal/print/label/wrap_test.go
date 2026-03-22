package label

import "testing"

func TestWrapText(t *testing.T) {
	face, err := LoadFace("spleen", 13)
	if err != nil {
		t.Fatalf("LoadFace: %v", err)
	}
	tests := []struct {
		name     string
		text     string
		maxWidth int
		want     int // expected number of lines, -1 means ">1"
	}{
		{"short", "Hello", 200, 1},
		{"needs wrap", "This is a longer text that should wrap to multiple lines", 100, -1},
		{"empty", "", 200, 0},
		{"polish", "Zażółć gęślą jaźń", 200, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapText(tt.text, face, tt.maxWidth)
			if tt.want >= 0 && len(lines) != tt.want {
				t.Errorf("wrapText(%q, %d) = %d lines %v, want %d",
					tt.text, tt.maxWidth, len(lines), lines, tt.want)
			}
			if tt.want == -1 && len(lines) <= 1 {
				t.Errorf("wrapText(%q, %d) = %d lines, want >1",
					tt.text, tt.maxWidth, len(lines))
			}
		})
	}
}
