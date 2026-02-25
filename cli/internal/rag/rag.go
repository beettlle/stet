// Package rag provides RAG-lite symbol lookup: extract symbols from a diff
// hunk, look up definitions in the repo (grep/ctags-style), and return
// signature + docstring for injection into the review prompt. Sub-phase 6.8.
// Resolvers are registered per file extension so other languages can be added
// without changing the core flow.
package rag

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
)

// Definition holds one symbol definition: file, line, signature, and optional
// docstring for injection into the prompt.
type Definition struct {
	Symbol    string // Name of the symbol.
	File      string // Path relative to repo root.
	Line      int    // 1-based line number of the definition.
	Signature string // One or a few lines of declaration.
	Docstring string // Optional comment above or inline.
}

// ResolveOptions bounds symbol resolution: max definitions and optional token cap.
type ResolveOptions struct {
	MaxDefinitions int // max number of definitions to return (0 = no limit)
	MaxTokens      int // max tokens for combined definitions; 0 = no cap
}

// CallGraphResult holds upstream callers and downstream callees for the
// function containing a hunk. Reuses Definition for each entry.
type CallGraphResult struct {
	Callers []Definition
	Callees []Definition
}

// CallGraphOptions bounds call-graph resolution: max callers, max callees, and optional token cap.
type CallGraphOptions struct {
	CallersMax int // max call sites to return (0 = use default)
	CalleesMax int // max callees to return (0 = use default)
	MaxTokens  int // max tokens for the combined block; 0 = no cap
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

// ErrEmptyExtension is returned by RegisterResolver when ext is empty.
var ErrEmptyExtension = errors.New("rag: empty extension")

// RegisterResolver registers a resolver for the given file extension (e.g. ".go").
// Extensions should include the leading dot. Returns an error if ext is empty.
func RegisterResolver(ext string, r Resolver) error {
	if ext == "" {
		return ErrEmptyExtension
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[ext] = r
	return nil
}

// MustRegisterResolver calls RegisterResolver and panics on error. Use from init() where error return is not possible.
func MustRegisterResolver(ext string, r Resolver) {
	if err := RegisterResolver(ext, r); err != nil {
		panic(err)
	}
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

// CallGraphResolver resolves callers and callees for the function containing a hunk.
// Each language (Go, etc.) can implement this interface and register by extension.
type CallGraphResolver interface {
	ResolveCallGraph(ctx context.Context, repoRoot, filePath, hunkContent string, opts CallGraphOptions) (*CallGraphResult, error)
}

var (
	callGraphRegistry   = make(map[string]CallGraphResolver)
	callGraphRegistryMu sync.RWMutex
)

// RegisterCallGraphResolver registers a call-graph resolver for the given file extension (e.g. ".go").
func RegisterCallGraphResolver(ext string, r CallGraphResolver) error {
	if ext == "" {
		return ErrEmptyExtension
	}
	callGraphRegistryMu.Lock()
	defer callGraphRegistryMu.Unlock()
	callGraphRegistry[ext] = r
	return nil
}

// MustRegisterCallGraphResolver calls RegisterCallGraphResolver and panics on error.
func MustRegisterCallGraphResolver(ext string, r CallGraphResolver) {
	if err := RegisterCallGraphResolver(ext, r); err != nil {
		panic(err)
	}
}

// ResolveCallGraph returns callers (upstream) and callees (downstream) for the
// function containing the hunk. Dispatches by file extension; for non-Go or
// when no resolver is registered, returns (nil, nil) without error.
func ResolveCallGraph(ctx context.Context, repoRoot, filePath, hunkContent string, opts CallGraphOptions) (*CallGraphResult, error) {
	ext := filepath.Ext(filePath)
	if ext == "" {
		return nil, nil
	}
	callGraphRegistryMu.RLock()
	r, ok := callGraphRegistry[ext]
	callGraphRegistryMu.RUnlock()
	if !ok || r == nil {
		return nil, nil
	}
	return r.ResolveCallGraph(ctx, repoRoot, filePath, hunkContent, opts)
}
