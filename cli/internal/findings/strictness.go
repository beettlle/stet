// Package findings defines the schema for code review findings. This file
// implements strictness preset resolution for the abstention filter and FP kill list.

package findings

import (
	"fmt"
	"strings"
)

// Valid strictness values (case-insensitive). The "+" suffix means do not apply the FP kill list.
const (
	StrictnessStrict   = "strict"
	StrictnessDefault  = "default"
	StrictnessLenient  = "lenient"
	StrictnessStrictPlus   = "strict+"
	StrictnessDefaultPlus  = "default+"
	StrictnessLenientPlus  = "lenient+"
)

// Default abstention thresholds when strictness is "default".
const (
	DefaultMinConfidenceKeep             = 0.8
	DefaultMinConfidenceMaintainability  = 0.9
)

var validStrictness = map[string]struct{}{
	StrictnessStrict:        {},
	StrictnessDefault:       {},
	StrictnessLenient:       {},
	StrictnessStrictPlus:    {},
	StrictnessDefaultPlus:   {},
	StrictnessLenientPlus:   {},
}

// ResolveStrictness normalizes s and returns the abstention thresholds and whether
// to apply the FP kill list. Valid values: strict, default, lenient, strict+,
// default+, lenient+. The "+" presets use the same thresholds but applyFP is false.
// Invalid s returns an error.
func ResolveStrictness(s string) (minKeep, minMaint float64, applyFP bool, err error) {
	norm := strings.TrimSpace(strings.ToLower(s))
	if _, ok := validStrictness[norm]; !ok {
		return 0, 0, false, fmt.Errorf("invalid strictness %q: use strict, default, lenient, strict+, default+, or lenient+", s)
	}
	applyFP = !strings.HasSuffix(norm, "+")
	base := strings.TrimSuffix(norm, "+")
	switch base {
	case "strict":
		return 0.6, 0.7, applyFP, nil
	case "default":
		return DefaultMinConfidenceKeep, DefaultMinConfidenceMaintainability, applyFP, nil
	case "lenient":
		return 0.9, 0.95, applyFP, nil
	default:
		panic("unreachable: base validated by validStrictness")
	}
}
