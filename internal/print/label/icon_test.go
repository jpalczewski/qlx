package label

import (
	"image"
	"testing"
)

func TestRasterizeIcon(t *testing.T) {
	// Clear cache between test runs
	iconCache.Range(func(key, _ any) bool {
		iconCache.Delete(key)
		return true
	})

	t.Run("valid icon returns correct size", func(t *testing.T) {
		img, err := RasterizeIcon("package", 24)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img == nil {
			t.Fatal("expected non-nil image")
		}
		bounds := img.Bounds()
		if bounds.Dx() != 24 || bounds.Dy() != 24 {
			t.Errorf("expected 24x24 image, got %dx%d", bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("empty name returns nil", func(t *testing.T) {
		img, err := RasterizeIcon("", 24)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img != nil {
			t.Error("expected nil image for empty name")
		}
	})

	t.Run("nonexistent icon returns error", func(t *testing.T) {
		img, err := RasterizeIcon("nonexistent-icon-xyz", 24)
		if err == nil {
			t.Fatal("expected error for nonexistent icon")
		}
		if img != nil {
			t.Error("expected nil image on error")
		}
	})

	t.Run("cached result returns same pointer", func(t *testing.T) {
		// Clear cache first
		iconCache.Range(func(key, _ any) bool {
			iconCache.Delete(key)
			return true
		})

		img1, err := RasterizeIcon("package", 16)
		if err != nil {
			t.Fatalf("first call: unexpected error: %v", err)
		}

		img2, err := RasterizeIcon("package", 16)
		if err != nil {
			t.Fatalf("second call: unexpected error: %v", err)
		}

		// Compare underlying pointers via the concrete type
		rgba1, ok1 := img1.(*image.RGBA)
		rgba2, ok2 := img2.(*image.RGBA)
		if !ok1 || !ok2 {
			t.Fatal("expected *image.RGBA type")
		}
		if rgba1 != rgba2 {
			t.Error("expected same pointer for cached icon")
		}
	})
}
