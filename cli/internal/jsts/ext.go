// Package jsts holds the single source of truth for which file extensions
// stet treats as JavaScript/TypeScript. Both hunk expansion (expand) and RAG
// symbol resolution (rag/js) consult this list so they cannot drift apart.
package jsts

import "slices"

// Extensions is the ordered list of JavaScript/TypeScript file extensions
// (including the leading dot) recognized by stet. Callers must not modify it.
var Extensions = []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"}

// IsExt reports whether ext (as returned by filepath.Ext, including the
// leading dot) is a JavaScript/TypeScript extension. Matching is exact and
// case-sensitive.
func IsExt(ext string) bool {
	return slices.Contains(Extensions, ext)
}
