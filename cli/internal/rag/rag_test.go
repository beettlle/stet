package rag

import (
	"context"
	"errors"
	"testing"
)

// mockResolver returns a fixed definition for any input (used to test dispatcher path).
type mockResolver struct{}

func (mockResolver) ResolveSymbols(ctx context.Context, repoRoot, filePath, hunkContent string, opts ResolveOptions) ([]Definition, error) {
	return []Definition{
		{Symbol: "Foo", File: "a.go", Line: 1, Signature: "func Foo() {}", Docstring: ""},
	}, nil
}

func TestResolveSymbols_unknownExtension_returnsNil(t *testing.T) {
	ctx := context.Background()
	defs, err := ResolveSymbols(ctx, "/repo", "src/file.xyz", "some content", ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if defs != nil {
		t.Errorf("expected nil for unknown extension; got %d definitions", len(defs))
	}
}

func TestResolveSymbols_emptyExtension_returnsNil(t *testing.T) {
	ctx := context.Background()
	defs, err := ResolveSymbols(ctx, "/repo", "Makefile", "content", ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if defs != nil {
		t.Errorf("expected nil for no extension; got %d definitions", len(defs))
	}
}

func TestRegisterResolver_emptyExtension_returnsError(t *testing.T) {
	err := RegisterResolver("", mockResolver{})
	if err == nil {
		t.Fatal("RegisterResolver with empty extension: want error, got nil")
	}
	if !errors.Is(err, ErrEmptyExtension) {
		t.Errorf("RegisterResolver: want errors.Is(..., ErrEmptyExtension); got %v", err)
	}
}

func TestResolveSymbols_withRegisteredResolver_returnsDefinitions(t *testing.T) {
	MustRegisterResolver(".got", mockResolver{})
	defer func() {
		registryMu.Lock()
		delete(registry, ".got")
		registryMu.Unlock()
	}()
	ctx := context.Background()
	defs, err := ResolveSymbols(ctx, "/repo", "pkg/x.got", "Foo()", ResolveOptions{MaxDefinitions: 5})
	if err != nil {
		t.Fatalf("ResolveSymbols: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition; got %d", len(defs))
	}
	if defs[0].Symbol != "Foo" || defs[0].Signature != "func Foo() {}" {
		t.Errorf("unexpected definition: %+v", defs[0])
	}
}
