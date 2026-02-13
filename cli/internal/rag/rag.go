// Package rag provides RAG-lite symbol lookup: extract symbols from a diff
// hunk, look up definitions in the repo (grep/ctags-style), and return
// signature + docstring for injection into the review prompt. Sub-phase 6.8.
// Resolvers are registered per file extension so other languages can be added
// without changing the core flow.
package rag

import (
	"context"
	"path/filepath"
	"sync"
)

// Definition holds one symbol definition: file, line, signature, and optional
// docstring for injection into the prompt.
type Definition struct {
	Symbol    string // name of the symbol
	File      string // path relative to repo root
	Line      int
	Signature string // one or a few lines of declaration
	Docstring string // optional comment above or inline
}

// ResolveOptions bounds symbol resolution: max definitions and optional token cap.
type ResolveOptions struct {
	MaxDefinitions int // max number of definitions to return (0 = no limit)
	MaxTokens      int // max tokens for combined definitions; 0 = no cap
}

// Resolver looks up symbols used in a hunk and returns their definitions.
// Each language (Go, TypeScript, etc.) implements this interface.
type Resolver interface {
	ResolveSymbols(ctx context.Context, repoRoot, filePath, hunkContent string, opts ResolveOptions) ([]Definition, error)
}

var (
	registry   = make(map[string]Resolver)
	registryMu sync.RWMutex
)

// RegisterResolver registers a resolver for the given file extension (e.g. ".go").
// Extensions should include the leading dot. Panics if ext is empty.
// Typically called from init() in language-specific packages.
func RegisterResolver(ext string, r Resolver) {
	if ext == "" {
		panic("rag: empty extension")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[ext] = r
}

// ResolveSymbols dispatches to the resolver for the given file's extension.
// Returns up to opts.MaxDefinitions definitions; total size may be capped by
// opts.MaxTokens. If no resolver is registered for the extension, returns
// (nil, nil). repoRoot and filePath are used by the resolver; hunkContent
// is the raw or context content of the diff hunk.
func ResolveSymbols(ctx context.Context, repoRoot, filePath, hunkContent string, opts ResolveOptions) ([]Definition, error) {
	ext := filepath.Ext(filePath)
	if ext == "" {
		return nil, nil
	}
	registryMu.RLock()
	r, ok := registry[ext]
	registryMu.RUnlock()
	if !ok || r == nil {
		return nil, nil
	}
	return r.ResolveSymbols(ctx, repoRoot, filePath, hunkContent, opts)
}
