package jsts

import (
	"slices"
	"testing"
)

func TestExtensions_orderedList(t *testing.T) {
	want := []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}
	if !slices.Equal(Extensions, want) {
		t.Fatalf("Extensions = %v, want %v", Extensions, want)
	}
}

func TestIsExt(t *testing.T) {
	for _, ext := range Extensions {
		if !IsExt(ext) {
			t.Errorf("IsExt(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{"", ".go", ".py", ".JS", ".Jsx", ".TSX", "js", ".mts", ".cts", ".json", ".jsx.bak"} {
		if IsExt(ext) {
			t.Errorf("IsExt(%q) = true, want false", ext)
		}
	}
}
