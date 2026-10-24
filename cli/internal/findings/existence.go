// Package findings defines the schema for code review findings. This file
// implements the existence-claim post-filter (issue #33): findings whose
// message claims a symbol is unused, undefined, not defined, not imported,
// not declared, missing an import, or not exported are dropped when the
// finding's own post-change file content proves otherwise. The filter is pure
// (no I/O) and conservative: a dropped true positive is worse than a kept
// false positive, so anything inconclusive keeps the finding.

package findings

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Existence-claim detection patterns. All matches are case-insensitive.
var (
	// missingClaimRegex matches claims that a symbol does not exist in the
	// file: undefined, not/never defined, not/never imported, not/never
	// declared, missing import(s), "without importing", or a would-be
	// ReferenceError. Value-uses of "undefined" (e.g. "returns undefined")
	// are stripped by undefinedValueUseRegex before this check.
	missingClaimRegex = regexp.MustCompile(`(?i)\b(?:undefined|not\s+defined|never\s+defined|not\s+imported|never\s+imported|not\s+declared|never\s+declared|missing\s+imports?|without\s+importing|referenceerror)\b`)
	// unusedClaimRegex matches claims that a symbol is unused.
	unusedClaimRegex = regexp.MustCompile(`(?i)\b(?:unused|not\s+used|never\s+used)\b`)
	// notExportedClaimRegex matches claims that a symbol is not exported.
	notExportedClaimRegex = regexp.MustCompile(`(?i)\bnot\s+exported\b`)
	// undefinedValueUseRegex matches value-uses of "undefined" (a function
	// returning / leaving / staying undefined is not an existence claim
	// about a named symbol); matched spans are removed before claim
	// detection so such messages do not trigger the filter.
	undefinedValueUseRegex = regexp.MustCompile(`(?i)\b(?:returns?|returning|yields?|yielding|produces?|gives?|stays?|remains?|leaves?|left|sets?|becomes?)\s+(?:\w+\s+){0,2}undefined\b`)
)

// backtickSpanRegex extracts `...` spans from a finding message.
var backtickSpanRegex = regexp.MustCompile("`([^`\\n]+)`")

// validIdentifierRegex reports whether a backticked span is a plain
// identifier (no dots, slashes, or other qualification).
var validIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// signatureLineRegex captures the remainder of lines that look like a
// declaration signature (func/function/def/constructor) so parameter lists
// can be inspected.
var signatureLineRegex = regexp.MustCompile(`(?m)(?:func(?:tion)?|def|constructor)\b[^\n]*`)

// parenGroupRegex captures one (...) group's inner text.
var parenGroupRegex = regexp.MustCompile(`\(([^()]*)\)`)

// importPatterns returns regexes proving ident is imported or required in a
// file, one entry per supported form. q must be regexp.QuoteMeta(ident).
func importPatterns(q string) []string {
	return []string{
		// JS/TS: import fs from "x"; import * as fs from "x"; import type fs from "x";
		// TS import-assignment: import fs = require("x")
		`(?im)\bimport\s+(?:type\s+)?(?:\*\s+as\s+)?` + q + `\b`,
		// JS/TS named imports: import { a, fs } from "x"
		`(?im)\bimport\s*\{[^}]*\b` + q + `\b[^}]*\}`,
		// CommonJS / dynamic: const fs = require("x"), const { fs } = require("x"),
		// const fs = await import("x")
		`(?im)\b(?:const|let|var)\s+(?:\{[^}]*\b` + q + `\b[^}]*|` + q + `)\s*=\s*(?:await\s+)?(?:require|import)\s*\(`,
		// Go: a line that is only a quoted import specifier (optionally
		// aliased or blank), covering grouped imports: import "fmt",
		// f "path/fs", _ "embed"
		`(?m)^\s*(?:[_\p{L}\d]+\s+)?"[^"]*` + q + `"\s*$`,
		// Python: import fs
		`(?im)^\s*import\s+` + q + `\b`,
		// Python: from mod import fs (incl. "from . import fs" and lists)
		`(?im)\bfrom\s+[\p{L}\d_.]+\s+import\s+[^#\n]*\b` + q + `\b`,
	}
}

// declarationPatterns returns regexes proving ident is declared in a file.
func declarationPatterns(q string) []string {
	return []string{
		// JS/TS: function fs(, function* fs(, async function fs(, const/let/var fs,
		// class fs, interface fs, enum fs, type fs
		`(?m)(?:^|[^\p{L}\d_.$])(?:async\s+)?(?:function\*?|const|let|var|class|interface|enum|type)\s+` + q + `\b`,
		// Go: func fs(, func (r R) fs(, var fs, const fs, type fs
		`(?m)(?:^|[^\p{L}\d_.$])(?:func\s+(?:\([^)]*\)\s*)?|var\s+|const\s+|type\s+)` + q + `\b`,
		// Python: def fs(, async def fs(, class fs
		`(?m)(?:^|[^\p{L}\d_.$])(?:async\s+)?(?:def|class)\s+` + q + `\b`,
	}
}

// compileAll compiles every pattern; patterns are static strings so
// MustCompile cannot panic.
func compileAll(patterns []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		res[i] = regexp.MustCompile(p)
	}
	return res
}

// matchesAny reports whether s matches any of res.
func matchesAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// identImported reports whether ident appears in an import/require form in
// content.
func identImported(content, ident string) bool {
	return matchesAny(compileAll(importPatterns(regexp.QuoteMeta(ident))), content)
}

// identDeclared reports whether ident is declared in content: a
// function/const/let/var/class/type/interface/enum (JS/TS), func/var/const/
// type including methods (Go), or def/class (Python).
func identDeclared(content, ident string) bool {
	return matchesAny(compileAll(declarationPatterns(regexp.QuoteMeta(ident))), content)
}

// identParameter reports whether ident occurs inside a parameter (or
// receiver, or named-result) list of a declaration signature in content.
// Single-line signatures only; multi-line parameter lists keep the finding
// (conservative).
func identParameter(content, ident string) bool {
	q, err := regexp.Compile(`\b` + regexp.QuoteMeta(ident) + `\b`)
	if err != nil {
		return false
	}
	for _, sig := range signatureLineRegex.FindAllString(content, -1) {
		for _, pg := range parenGroupRegex.FindAllStringSubmatch(sig, -1) {
			if q.MatchString(pg[1]) {
				return true
			}
		}
	}
	return false
}

// identProvenInFile reports whether ident is imported, declared, or a
// parameter of a declaration in content — the proof required to drop a
// "not imported / undefined / not defined / not declared / ReferenceError"
// claim.
func identProvenInFile(content, ident string) bool {
	return identImported(content, ident) || identDeclared(content, ident) || identParameter(content, ident)
}

// identUsedTwiceOutsideDecl reports whether ident occurs at least twice as a
// whole word in content outside lines that declare or import it — the proof
// required to drop an "unused" claim.
func identUsedTwiceOutsideDecl(content, ident string) bool {
	q := regexp.QuoteMeta(ident)
	wordRe := regexp.MustCompile(`\b` + q + `\b`)
	declRes := compileAll(declarationPatterns(q))
	importRes := compileAll(importPatterns(q))
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if !wordRe.MatchString(line) {
			continue
		}
		if matchesAny(declRes, line) || matchesAny(importRes, line) {
			continue
		}
		count += len(wordRe.FindAllString(line, -1))
	}
	return count >= 2
}

// identExported reports whether content exports ident: a JS/TS export
// statement or CommonJS exports assignment naming ident, or (Go) a declared
// identifier with an uppercase initial. Python __all__ is not consulted;
// such claims keep the finding (conservative).
func identExported(content, ident string) bool {
	q := regexp.QuoteMeta(ident)
	patterns := []string{
		// JS/TS: export (default)? (async)? function/const/... ident
		`(?im)\bexport\s+(?:default\s+)?(?:async\s+)?(?:function\*?|const|let|var|class|type|interface|enum)\s+` + q + `\b`,
		// JS/TS: export default ident
		`(?im)\bexport\s+default\s+` + q + `\b`,
		// JS/TS: export { a, ident }; export type { ident }
		`(?im)\bexport\s+(?:type\s+)?\{[^}]*\b` + q + `\b[^}]*\}`,
		// CommonJS: module.exports = { ident } / exports.ident = ...
		`(?im)\b(?:module\.)?exports\b[^;\n]*\b` + q + `\b`,
	}
	if matchesAny(compileAll(patterns), content) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(ident)
	if !unicode.IsUpper(r) {
		return false
	}
	return identDeclared(content, ident)
}

// looksLikeModuleRef reports whether a non-identifier backticked span refers
// to another module (a path or package specifier): "./utils", "../x",
// "@scope/pkg", "utils.js".
func looksLikeModuleRef(span string) bool {
	return strings.ContainsAny(span, `/\@`) || strings.Contains(span, ".")
}

// FilterExistenceClaims drops findings whose message claims a symbol is
// missing, unused, undefined, not imported, not declared, or not exported
// when fileContent — the post-change content of the finding's file — proves
// otherwise. Per claim type: "not imported / undefined / not defined / not
// declared / ReferenceError" requires every backticked identifier to be
// imported, declared, or a parameter of a declaration; "unused" requires at
// least two whole-word occurrences outside declaration/import lines; "not
// exported" requires the file to export the identifier. Findings are kept
// when the message has no existence claim, no identifier can be extracted,
// fileContent is empty, or the claim is about another module (e.g. "not
// exported from `./utils`"): other modules are never read. Returns a new
// slice; the input is not modified. Order of kept findings is preserved.
func FilterExistenceClaims(batch []Finding, fileContent string) []Finding {
	if len(batch) == 0 {
		return nil
	}
	if strings.TrimSpace(fileContent) == "" {
		return batch
	}
	out := make([]Finding, 0, len(batch))
	for _, f := range batch {
		if existenceClaimDisproven(f.Message, fileContent) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// existenceClaimDisproven reports whether every existence claim in msg about
// every backticked identifier is disproven by content. All claim types
// present must be disproven for the finding to be dropped.
func existenceClaimDisproven(msg, content string) bool {
	// Strip value-uses of "undefined" so "returns undefined" and friends are
	// not mistaken for existence claims.
	msg = undefinedValueUseRegex.ReplaceAllString(msg, "")
	hasMissing := missingClaimRegex.MatchString(msg)
	hasUnused := unusedClaimRegex.MatchString(msg)
	hasNotExported := notExportedClaimRegex.MatchString(msg)
	if !hasMissing && !hasUnused && !hasNotExported {
		return false
	}
	var idents []string
	for _, m := range backtickSpanRegex.FindAllStringSubmatch(msg, -1) {
		span := m[1]
		if validIdentifierRegex.MatchString(span) {
			idents = append(idents, span)
			continue
		}
		// The claim names another module (e.g. "not exported from `./utils`"):
		// the filter never reads other modules, so keep the finding.
		if looksLikeModuleRef(span) {
			return false
		}
	}
	if len(idents) == 0 {
		return false
	}
	if hasMissing {
		for _, id := range idents {
			if !identProvenInFile(content, id) {
				return false
			}
		}
	}
	if hasUnused {
		for _, id := range idents {
			if !identUsedTwiceOutsideDecl(content, id) {
				return false
			}
		}
	}
	if hasNotExported {
		for _, id := range idents {
			if !identExported(content, id) {
				return false
			}
		}
	}
	return true
}
