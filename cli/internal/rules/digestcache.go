// On-disk cache of rule review digests, invalidated when a rule file changes.

package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// DigestCacheFileName is the cache file name inside the stet state directory.
const DigestCacheFileName = "rules-digest-cache.json"

// digestCacheVersion changes whenever BuildDigest output changes, so digests
// built by an older stet are rebuilt instead of reused.
const digestCacheVersion = 1

type digestEntry struct {
	Size      int64  `json:"size"`
	ModTimeNs int64  `json:"mod_time_ns"`
	SHA256    string `json:"sha256"`
	Digest    string `json:"digest"`
}

type digestCacheFile struct {
	Version int                    `json:"version"`
	Entries map[string]digestEntry `json:"entries"`
}

// DigestCache maps rule file paths to their review digests. An entry is reused
// while the file's size and modification time match; otherwise the content
// hash decides between refreshing the stamp and rebuilding the digest. A nil
// *DigestCache builds every digest without caching.
type DigestCache struct {
	mu      sync.Mutex
	path    string // empty = in-memory only
	entries map[string]digestEntry
	dirty   bool
}

// OpenDigestCache loads the digest cache from stateDir. A missing file starts
// an empty cache; an unreadable, corrupt, or older-version file is logged and
// replaced on the next Save. An empty stateDir yields an in-memory cache.
func OpenDigestCache(stateDir string) *DigestCache {
	c := &DigestCache{entries: make(map[string]digestEntry)}
	if stateDir == "" {
		return c
	}
	c.path = filepath.Join(stateDir, DigestCacheFileName)
	data, err := os.ReadFile(c.path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("rules: digest cache %s unreadable, rebuilding: %v", c.path, err)
			c.dirty = true
		}
		return c
	}
	var file digestCacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("rules: digest cache %s corrupt, rebuilding: %v", c.path, err)
		c.dirty = true
		return c
	}
	if file.Version != digestCacheVersion {
		c.dirty = true
		return c
	}
	for k, v := range file.Entries {
		c.entries[k] = v
	}
	return c
}

// digestFor returns the review digest for rule, whose file has the given size,
// modification time, and content.
func (c *DigestCache) digestFor(rule CursorRule, size, modTimeNs int64, content []byte) string {
	if c == nil {
		return BuildDigest(rule)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := rule.Path
	entry, ok := c.entries[key]
	if ok && entry.Size == size && entry.ModTimeNs == modTimeNs {
		return entry.Digest
	}
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	if ok && entry.SHA256 == hash {
		entry.Size, entry.ModTimeNs = size, modTimeNs
		c.entries[key] = entry
		c.dirty = true
		return entry.Digest
	}
	digest := BuildDigest(rule)
	c.entries[key] = digestEntry{Size: size, ModTimeNs: modTimeNs, SHA256: hash, Digest: digest}
	c.dirty = true
	return digest
}

// pruneDir removes entries for files in dir that are not in keep (deleted or
// no longer valid rule files).
func (c *DigestCache) pruneDir(dir string, keep []CursorRule) {
	if c == nil {
		return
	}
	kept := make(map[string]bool, len(keep))
	for _, r := range keep {
		kept[r.Path] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for path := range c.entries {
		if filepath.Dir(path) == dir && !kept[path] {
			delete(c.entries, path)
			c.dirty = true
		}
	}
}

// retainDirs removes entries whose rules directory is not in dirs (the
// directory itself was removed from the repository).
func (c *DigestCache) retainDirs(dirs []RulesDir) {
	if c == nil {
		return
	}
	live := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		live[d.AbsDir] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for path := range c.entries {
		if !live[filepath.Dir(path)] {
			delete(c.entries, path)
			c.dirty = true
		}
	}
}

// Save writes the cache when it changed since it was opened. The file is
// written to a temporary file and renamed so readers never see a partial cache.
func (c *DigestCache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty || c.path == "" {
		return nil
	}
	data, err := json.Marshal(digestCacheFile{Version: digestCacheVersion, Entries: c.entries})
	if err != nil {
		return fmt.Errorf("rules: encode digest cache: %w", err)
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("rules: create digest cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".rules-digest-cache-*.tmp")
	if err != nil {
		return fmt.Errorf("rules: create digest cache temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("rules: write digest cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rules: close digest cache: %w", err)
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rules: replace digest cache: %w", err)
	}
	c.dirty = false
	return nil
}

// isRuleFile reports whether e is a regular .mdc rule file.
func isRuleFile(e fs.DirEntry) bool {
	return !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mdc")
}

// writeStamp appends one rule file's name, size, and modification time to fp.
func writeStamp(fp *strings.Builder, name string, info fs.FileInfo) {
	fp.WriteString(name)
	fp.WriteByte('|')
	fp.WriteString(strconv.FormatInt(info.Size(), 10))
	fp.WriteByte('|')
	fp.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
	fp.WriteByte('\n')
}

// dirFingerprint returns the name, size, and modification time of every rule
// file in dir without reading file contents. Any added, removed, or modified
// rule file changes the fingerprint.
func dirFingerprint(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var fp strings.Builder
	for _, e := range entries {
		if !isRuleFile(e) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		writeStamp(&fp, e.Name(), info)
	}
	return fp.String()
}
