// Package findings defines the schema for code review findings. This file
// implements the Phase 6.5 false positive (FP) kill list: findings whose
// message matches a banned phrase are suppressed regardless of confidence.

package findings

import (
	"regexp"
	"sync"
)

// defaultBannedPhrases is the built-in list of phrases that indicate
// low-value or noisy findings. Matches are case-insensitive substring.
var defaultBannedPhrases = []string{
	"Consider adding comments",
	"Consider adding a comment",
	"Ensure that...",
	"It might be beneficial",
	"You might want to",
	"it may be beneficial",
	"consider adding documentation",
}

var (
	defaultCompiledOnce sync.Once
	defaultCompiled      []*regexp.Regexp
)

func initCompiled() {
	defaultCompiledOnce.Do(func() {
		defaultCompiled = make([]*regexp.Regexp, 0, len(defaultBannedPhrases))
		for _, phrase := range defaultBannedPhrases {
			// Literal substring, case-insensitive; QuoteMeta so no regex syntax in list.
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(phrase))
			defaultCompiled = append(defaultCompiled, re)
		}
	})
}

// FilterFPKillList post-processes findings by dropping any whose Message
// matches a banned phrase (Phase 6.5 FP kill list). Suppression is regardless
// of confidence. Returns a new slice; the input is not modified. Order of
// kept findings is preserved.
func FilterFPKillList(list []Finding) []Finding {
	if len(list) == 0 {
		return nil
	}
	initCompiled()
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if matchesBannedPhrase(f.Message) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// matchesBannedPhrase returns true if msg contains any default banned phrase
// (case-insensitive). Caller must have run initCompiled().
func matchesBannedPhrase(msg string) bool {
	for _, re := range defaultCompiled {
		if re.MatchString(msg) {
			return true
		}
	}
	return false
}
