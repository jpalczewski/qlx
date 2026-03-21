package palette

import "testing"

func TestValidColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{"valid red", "red", true},
		{"valid teal", "teal", true},
		{"invalid", "neon", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidColor(tt.color); got != tt.want {
				t.Errorf("ValidColor(%q) = %v, want %v", tt.color, got, tt.want)
			}
		})
	}
}

func TestRandomColor(t *testing.T) {
	c := RandomColor()
	if c.Name == "" || c.Hex == "" {
		t.Errorf("RandomColor() returned empty: %+v", c)
	}
	if !ValidColor(c.Name) {
		t.Errorf("RandomColor() returned invalid color: %s", c.Name)
	}
}

func TestColorByName(t *testing.T) {
	c, ok := ColorByName("blue")
	if !ok {
		t.Fatal("ColorByName(blue) not found")
	}
	if c.Hex != "#4d9de0" {
		t.Errorf("ColorByName(blue).Hex = %s, want #4d9de0", c.Hex)
	}
	_, ok = ColorByName("nonexistent")
	if ok {
		t.Error("ColorByName(nonexistent) should return false")
	}
}

func TestAllColors(t *testing.T) {
	colors := AllColors()
	if len(colors) != 10 {
		t.Errorf("AllColors() len = %d, want 10", len(colors))
	}
}
