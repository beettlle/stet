// ID helpers for human-facing display and prefix resolution (Git-style short IDs).

package findings

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// ShortIDDisplayLen is the number of hex characters shown for finding IDs in human output (e.g. list, status --ids).
	ShortIDDisplayLen = 7
	// MinPrefixLen is the minimum number of characters required when resolving a finding ID by prefix (e.g. dismiss).
	MinPrefixLen = 4
)

// ShortID returns an abbreviated form of id for human-facing display. If id is longer than ShortIDDisplayLen,
// returns the first ShortIDDisplayLen characters; otherwise returns id unchanged.
func ShortID(id string) string {
	if len(id) <= ShortIDDisplayLen {
		return id
	}
	return id[:ShortIDDisplayLen]
}

// ErrFindingIDTooShort is returned when the prefix length is below MinPrefixLen.
var ErrFindingIDTooShort = errors.New("finding id must be at least 4 characters")

// ResolveFindingIDByPrefix finds the single finding in list whose ID has the given prefix (case-insensitive).
// prefix is matched with strings.HasPrefix against each finding's full ID. Returns the full ID when exactly
// one finding matches; otherwise returns an error (not found or ambiguous).
func ResolveFindingIDByPrefix(list []Finding, prefix string) (fullID string, err error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", errors.New("finding id is required")
	}
	if len(prefix) < MinPrefixLen {
		return "", ErrFindingIDTooShort
	}
	prefixLower := strings.ToLower(prefix)
	var matches []string
	for _, f := range list {
		if f.ID == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(f.ID), prefixLower) {
			matches = append(matches, f.ID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no finding with id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous id %q; use more characters", prefix)
	}
}
