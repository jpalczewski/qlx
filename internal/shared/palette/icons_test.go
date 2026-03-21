package palette

import "testing"

func TestValidIcon(t *testing.T) {
	tests := []struct {
		name string
		icon string
		want bool
	}{
		{"valid wrench", "wrench", true},
		{"valid cpu", "cpu", true},
		{"invalid", "unicorn", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidIcon(tt.icon); got != tt.want {
				t.Errorf("ValidIcon(%q) = %v, want %v", tt.icon, got, tt.want)
			}
		})
	}
}

func TestRandomIcon(t *testing.T) {
	ic := RandomIcon()
	if ic.Name == "" || ic.Category == "" {
		t.Errorf("RandomIcon() returned empty: %+v", ic)
	}
	if !ValidIcon(ic.Name) {
		t.Errorf("RandomIcon() returned invalid icon: %s", ic.Name)
	}
}

func TestIconByName(t *testing.T) {
	ic, ok := IconByName("wrench")
	if !ok {
		t.Fatal("IconByName(wrench) not found")
	}
	if ic.Category == "" {
		t.Error("IconByName(wrench).Category is empty")
	}
	_, ok = IconByName("nonexistent")
	if ok {
		t.Error("IconByName(nonexistent) should return false")
	}
}

func TestIconCategories(t *testing.T) {
	cats := IconCategories()
	if len(cats) == 0 {
		t.Error("IconCategories() is empty")
	}
	for _, cat := range cats {
		if cat.Name == "" || len(cat.Icons) == 0 {
			t.Errorf("empty category: %+v", cat)
		}
	}
}

func TestAllIcons(t *testing.T) {
	icons := AllIcons()
	if len(icons) < 50 {
		t.Errorf("AllIcons() len = %d, want >= 50", len(icons))
	}
}
