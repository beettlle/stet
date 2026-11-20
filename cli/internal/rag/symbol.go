// symbol.go provides batched symbol lookup for RAG resolvers: instead of
// spawning one `git grep` process per symbol (up to maxSymbolCandidates=30
// per hunk), all symbol patterns are looked up in a single batched grep and
// the hits are mapped back to their symbols.
package rag

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"stet/cli/internal/git"
)

// SymbolPattern pairs a symbol name with its language-specific grep pattern
// (POSIX ERE, as accepted by `git grep -E` and Go's regexp).
type SymbolPattern struct {
	Symbol  string // Symbol being looked up (e.g. function or type name).
	Pattern string // ERE matching that symbol's definition lines.
}

// SymbolMatch is the first grep hit found for a symbol's pattern.
type SymbolMatch struct {
	Symbol  string // Symbol the hit was mapped back to.
	RelPath string // Repo-relative path of the defining file (slash-separated).
	AbsPath string // Absolute path of the defining file.
	Line    int    // 1-based line number of the definition.
	Content string // Matched definition line.
}

// BatchGrepDefinitions looks up all patterns with a single batched
// `git grep -n -E -e p1 -e p2 ...` process instead of one process per
// pattern. Hits are mapped back to symbols by re-matching each hit's content
// against each compiled pattern: for every symbol the first matching hit in
// git's output order wins, which is exactly the result a per-symbol grep
// would have returned (git's traversal order does not depend on the number
// of -e patterns).
//
// The returned slice is aligned with pairs: results[i] corresponds to
// pairs[i], and a nil entry means no line matched that symbol's pattern.
func BatchGrepDefinitions(ctx context.Context, repoRoot string, pairs []SymbolPattern) ([]*SymbolMatch, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	compiled := make([]*regexp.Regexp, len(pairs))
	patterns := make([]string, len(pairs))
	for i, p := range pairs {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rag: compile pattern for symbol %q: %w", p.Symbol, err)
		}
		compiled[i] = re
		patterns[i] = p.Pattern
	}
	hits, err := git.Grep(ctx, repoRoot, patterns)
	if err != nil {
		return nil, fmt.Errorf("rag: batched grep of %d symbols: %w", len(pairs), err)
	}
	results := make([]*SymbolMatch, len(pairs))
	for i, re := range compiled {
		for _, h := range hits {
			if !re.MatchString(h.Content) {
				continue
			}
			results[i] = &SymbolMatch{
				Symbol:  pairs[i].Symbol,
				RelPath: h.Path,
				AbsPath: filepath.Join(repoRoot, filepath.FromSlash(h.Path)),
				Line:    h.Line,
				Content: h.Content,
			}
			break
		}
	}
	return results, nil
}
