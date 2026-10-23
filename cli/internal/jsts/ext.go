// Package jsts holds the single source of truth for which file extensions
// stet treats as JavaScript/TypeScript. Both hunk expansion (expand) and RAG
// symbol resolution (rag/js) consult this list so they cannot drift apart.
package jsts

import "slices"

// extensions is the ordered list of JavaScript/TypeScript file extensions
// (including the leading dot) recognized by stet. It is unexported so callers
// cannot modify it; use Extensions to read a copy.
var extensions = []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}

// Extensions returns a copy of the ordered list of JavaScript/TypeScript file
// extensions (including the leading dot) recognized by stet. Mutating the
// returned slice does not affect later calls or IsExt.
func Extensions() []string {
	return slices.Clone(extensions)
}

// IsExt reports whether ext (as returned by filepath.Ext, including the
// leading dot) is a JavaScript/TypeScript extension. Matching is exact and
// case-sensitive.
func IsExt(ext string) bool {
	return slices.Contains(extensions, ext)
}
