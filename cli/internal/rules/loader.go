// Nested .cursor/rules discovery and per-file loading.

package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RulesDir identifies a .cursor/rules directory and its path prefix for matching.
type RulesDir struct {
	// RelPath is the path from repo root to the parent of .cursor/rules (e.g. "" for root, "cli" for cli/.cursor/rules).
	RelPath string
	// AbsDir is the absolute path to the rules directory.
	AbsDir string
}

// DiscoverRulesDirs finds all .cursor/rules directories under repoRoot. It adds the root
// .cursor/rules first if present, then walks the tree and adds any nested .cursor/rules.
// RelPath is normalized to forward slashes; "." for repo root is returned as "".
// Skips descending into .git, vendor, and node_modules for performance.
func DiscoverRulesDirs(repoRoot string) ([]RulesDir, error) {
	var result []RulesDir
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == ".git" || base == "vendor" || base == "node_modules" {
			return filepath.SkipDir
		}
		if base == ".cursor" {
			rulesPath := filepath.Join(path, "rules")
			if info, e := os.Stat(rulesPath); e == nil && info.IsDir() {
				parent := filepath.Dir(path)
				rel, relErr := filepath.Rel(repoRoot, parent)
				if relErr != nil {
					return filepath.SkipDir
				}
				rel = filepath.ToSlash(rel)
				if rel == "." {
					rel = ""
				}
				result = append(result, RulesDir{RelPath: rel, AbsDir: rulesPath})
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i].RelPath, result[j].RelPath
		if a == "" {
			return true
		}
		if b == "" {
			return false
		}
		return a < b
	})
	return result, nil
}

// Loader caches discovered rules dirs and per-dir rule lists for fast per-file lookup.
// A cached dir is reloaded when its fingerprint (rule file names, sizes, and
// modification times) changes, so edits are picked up by long-lived loaders.
type Loader struct {
	dirs    []RulesDir
	cache   map[string]loadedDir
	digests *DigestCache
}

type loadedDir struct {
	rules       []CursorRule
	fingerprint string
}

// NewLoader discovers .cursor/rules dirs under repoRoot and returns a loader.
// If discovery fails, returns a loader with only the root .cursor/rules if it exists.
func NewLoader(repoRoot string) *Loader {
	dirs, err := DiscoverRulesDirs(repoRoot)
	if err != nil {
		rootRules := filepath.Join(repoRoot, ".cursor", "rules")
		if info, e := os.Stat(rootRules); e == nil && info.IsDir() {
			dirs = []RulesDir{{RelPath: "", AbsDir: rootRules}}
		} else {
			dirs = nil
		}
	}
	return &Loader{dirs: dirs, cache: make(map[string]loadedDir)}
}

// WithDigestCache makes the loader take review digests from c and drops cache
// entries for rules directories that no longer exist. Returns l for chaining.
func (l *Loader) WithDigestCache(c *DigestCache) *Loader {
	l.digests = c
	c.retainDirs(l.dirs)
	return l
}

// SaveDigests persists the digest cache when one is attached and it changed.
func (l *Loader) SaveDigests() error {
	return l.digests.Save()
}

// RulesForFile returns rules from root and from any nested .cursor/rules whose path
// is a prefix of filePath. FilePath is relative to repo root. Order is root first,
// then nested dirs by RelPath (lexicographic). Inference from description is already
// applied when rules are loaded (in parseMDC).
func (l *Loader) RulesForFile(filePath string) []CursorRule {
	if len(l.dirs) == 0 {
		return nil
	}
	filePathSlash := filepath.ToSlash(filePath)
	var applicable []RulesDir
	for _, d := range l.dirs {
		if d.RelPath == "" || filePathSlash == d.RelPath || strings.HasPrefix(filePathSlash, d.RelPath+"/") {
			applicable = append(applicable, d)
		}
	}
	if len(applicable) == 0 {
		return nil
	}
	var merged []CursorRule
	for _, d := range applicable {
		merged = append(merged, l.dirRules(d.AbsDir)...)
	}
	return merged
}

// dirRules returns the cached rules for dir, reloading them when the dir's
// fingerprint changed since they were loaded.
func (l *Loader) dirRules(dir string) []CursorRule {
	cached, ok := l.cache[dir]
	if ok && cached.fingerprint == dirFingerprint(dir) {
		return cached.rules
	}
	loaded, fingerprint := loadRulesDir(dir, l.digests)
	l.digests.pruneDir(dir, loaded)
	l.cache[dir] = loadedDir{rules: loaded, fingerprint: fingerprint}
	return loaded
}
