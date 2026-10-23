package jsts

import (
	"slices"
	"testing"
)

func TestExtensions_orderedList(t *testing.T) {
	want := []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}
	if got := Extensions(); !slices.Equal(got, want) {
		t.Fatalf("Extensions() = %v, want %v", got, want)
	}
}

func TestExtensions_mutationIsolation(t *testing.T) {
	original := Extensions()

	got := Extensions()
	// Assign an element.
	got[0] = ".mutated"
	// Append past capacity.
	got = append(got, ".extra")

	if again := Extensions(); !slices.Equal(again, original) {
		t.Fatalf("Extensions() after mutation = %v, want unchanged %v", again, original)
	}
	if IsExt(".mutated") {
		t.Error(`IsExt(".mutated") = true after caller mutation, want false`)
	}
	if IsExt(".extra") {
		t.Error(`IsExt(".extra") = true after caller append, want false`)
	}
	if !IsExt(".js") || !IsExt(".tsx") {
		t.Error("IsExt no longer recognizes built-in extensions after caller mutation")
	}
}

func TestIsExt(t *testing.T) {
	for _, ext := range Extensions() {
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
