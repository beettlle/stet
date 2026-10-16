package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cacheTestContent = "### H\n**Detect:** original"

func cacheTestRule(path, content string) CursorRule {
	return CursorRule{Source: filepath.Base(path), Path: path, Content: content}
}

func TestDigestCache_nilBuildsDigest(t *testing.T) {
	var c *DigestCache
	got := c.digestFor(cacheTestRule("/r/a.mdc", cacheTestContent), 1, 1, []byte(cacheTestContent))
	if !strings.Contains(got, "**Detect:** original") {
		t.Errorf("nil cache digestFor = %q", got)
	}
	if err := c.Save(); err != nil {
		t.Errorf("nil cache Save: %v", err)
	}
	c.pruneDir("/r", nil)
	c.retainDirs(nil)
}

func TestDigestCache_invalidation(t *testing.T) {
	const path = "/r/a.mdc"
	content := []byte(cacheTestContent)
	tests := []struct {
		name       string
		size, mod  int64
		content    string
		wantCached bool
	}{
		{name: "same_size_and_mtime_hits", size: 10, mod: 100, content: cacheTestContent, wantCached: true},
		{name: "mtime_changed_same_hash_hits", size: 10, mod: 200, content: cacheTestContent, wantCached: true},
		{name: "size_changed_same_hash_hits", size: 11, mod: 100, content: cacheTestContent, wantCached: true},
		{name: "content_changed_rebuilds", size: 12, mod: 300, content: "### H\n**Detect:** edited", wantCached: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := OpenDigestCache("")
			c.digestFor(cacheTestRule(path, cacheTestContent), 10, 100, content)
			entry := c.entries[path]
			entry.Digest = "SENTINEL"
			c.entries[path] = entry
			c.dirty = false

			got := c.digestFor(cacheTestRule(path, tt.content), tt.size, tt.mod, []byte(tt.content))
			if tt.wantCached {
				if got != "SENTINEL" {
					t.Errorf("digestFor = %q, want cached SENTINEL", got)
				}
			} else if !strings.Contains(got, "edited") {
				t.Errorf("digestFor = %q, want rebuilt digest", got)
			}
			after := c.entries[path]
			if after.Size != tt.size || after.ModTimeNs != tt.mod {
				t.Errorf("entry stamp = (%d, %d), want (%d, %d)", after.Size, after.ModTimeNs, tt.size, tt.mod)
			}
			sameStamp := tt.size == 10 && tt.mod == 100
			if c.dirty == sameStamp {
				t.Errorf("dirty = %v, want %v", c.dirty, !sameStamp)
			}
		})
	}
}

func TestDigestCache_saveAndReopen(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	c := OpenDigestCache(stateDir)
	c.digestFor(cacheTestRule("/r/a.mdc", cacheTestContent), 10, 100, []byte(cacheTestContent))
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if c.dirty {
		t.Error("dirty after Save, want false")
	}
	leftovers, _ := filepath.Glob(filepath.Join(stateDir, ".rules-digest-cache-*.tmp"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left after Save: %v", leftovers)
	}
	reopened := OpenDigestCache(stateDir)
	if got := reopened.entries["/r/a.mdc"].Digest; !strings.Contains(got, "original") {
		t.Errorf("reopened digest = %q", got)
	}
	if reopened.dirty {
		t.Error("reopened cache dirty, want clean")
	}
}

func TestDigestCache_saveCleanIsNoop(t *testing.T) {
	stateDir := t.TempDir()
	if err := OpenDigestCache(stateDir).Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, DigestCacheFileName)); !os.IsNotExist(err) {
		t.Errorf("clean Save wrote a file (stat err = %v)", err)
	}
}

func TestDigestCache_corruptOrOldVersionRebuilds(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "corrupt_json", data: "{not json"},
		{name: "old_version", data: `{"version":0,"entries":{"/r/a.mdc":{"digest":"stale"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := t.TempDir()
			cachePath := filepath.Join(stateDir, DigestCacheFileName)
			if err := os.WriteFile(cachePath, []byte(tt.data), 0o644); err != nil {
				t.Fatal(err)
			}
			c := OpenDigestCache(stateDir)
			if len(c.entries) != 0 || !c.dirty {
				t.Fatalf("entries = %d, dirty = %v; want empty and dirty", len(c.entries), c.dirty)
			}
			if err := c.Save(); err != nil {
				t.Fatalf("Save: %v", err)
			}
			data, err := os.ReadFile(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			var file digestCacheFile
			if err := json.Unmarshal(data, &file); err != nil || file.Version != digestCacheVersion {
				t.Errorf("rewritten cache = %s (err %v), want version %d", data, err, digestCacheVersion)
			}
		})
	}
}

func TestDigestCache_unreadableFileRebuilds(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(stateDir, DigestCacheFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	c := OpenDigestCache(stateDir)
	if !c.dirty || len(c.entries) != 0 {
		t.Errorf("dirty = %v, entries = %d; want dirty and empty", c.dirty, len(c.entries))
	}
	if err := c.Save(); err == nil {
		t.Error("Save over a directory: want error")
	}
}

func TestDigestCache_saveErrorWhenStateDirIsFile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(stateFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := OpenDigestCache(stateFile)
	c.digestFor(cacheTestRule("/r/a.mdc", cacheTestContent), 1, 1, []byte(cacheTestContent))
	if err := c.Save(); err == nil {
		t.Error("Save with state dir as a file: want error")
	}
}

func TestDigestCache_pruneAndRetain(t *testing.T) {
	c := OpenDigestCache("")
	for _, p := range []string{"/r/a.mdc", "/r/b.mdc", "/old/c.mdc"} {
		c.digestFor(cacheTestRule(p, cacheTestContent), 1, 1, []byte(cacheTestContent))
	}
	c.pruneDir("/r", []CursorRule{{Path: "/r/a.mdc"}})
	if _, ok := c.entries["/r/b.mdc"]; ok {
		t.Error("pruneDir kept deleted /r/b.mdc")
	}
	if _, ok := c.entries["/old/c.mdc"]; !ok {
		t.Error("pruneDir removed entry from another dir")
	}
	c.retainDirs([]RulesDir{{AbsDir: "/r"}})
	if _, ok := c.entries["/old/c.mdc"]; ok {
		t.Error("retainDirs kept entry for removed dir")
	}
	if _, ok := c.entries["/r/a.mdc"]; !ok {
		t.Error("retainDirs removed live entry")
	}
}

func TestLoader_digestCacheReloadsEditedAndDeletedRules(t *testing.T) {
	repo := t.TempDir()
	stateDir := filepath.Join(repo, ".review")
	mustCreateRulesDir(t, repo, "")
	rulesDir := filepath.Join(repo, ".cursor", "rules")
	goRule := filepath.Join(rulesDir, "go.mdc")
	extra := filepath.Join(rulesDir, "extra.mdc")
	writeRule := func(path, body string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte("---\nglobs: \"*.go\"\n---\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	writeRule(goRule, "### Errors\n**Detect:** swallowed error", base)
	writeRule(extra, "### Extra\n**Detect:** extra check", base)

	loader := NewLoader(repo).WithDigestCache(OpenDigestCache(stateDir))
	got := loader.RulesForFile("main.go")
	if len(got) != 2 || !strings.Contains(got[1].Digest, "swallowed error") {
		t.Fatalf("first load = %+v, want go.mdc digest", got)
	}
	if err := loader.SaveDigests(); err != nil {
		t.Fatalf("SaveDigests: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, DigestCacheFileName)); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}

	writeRule(goRule, "### Errors\n**Detect:** wrapped with %w", base.Add(time.Minute))
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	got = loader.RulesForFile("main.go")
	if len(got) != 1 || !strings.Contains(got[0].Digest, "wrapped with %w") {
		t.Fatalf("after edit and delete = %+v, want only edited go.mdc digest", got)
	}
	if err := loader.SaveDigests(); err != nil {
		t.Fatalf("SaveDigests: %v", err)
	}
	reopened := OpenDigestCache(stateDir)
	if _, ok := reopened.entries[extra]; ok {
		t.Error("cache kept entry for deleted extra.mdc")
	}
	if !strings.Contains(reopened.entries[goRule].Digest, "wrapped with %w") {
		t.Errorf("cache digest for go.mdc = %q, want edited", reopened.entries[goRule].Digest)
	}
}

func TestLoader_unchangedDirServedFromMemory(t *testing.T) {
	repo := t.TempDir()
	mustCreateRulesDir(t, repo, "")
	path := filepath.Join(repo, ".cursor", "rules", "a.mdc")
	if err := os.WriteFile(path, []byte("---\nalwaysApply: true\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(repo)
	if first := loader.RulesForFile("x.go"); len(first) != 1 {
		t.Fatalf("first load = %+v, want 1 rule", first)
	}
	dir := filepath.Dir(path)
	loader.cache[dir].rules[0].Content = "IN-MEMORY"
	if second := loader.RulesForFile("y.go"); len(second) != 1 || second[0].Content != "IN-MEMORY" {
		t.Errorf("unchanged dir: got %+v, want cached in-memory rule", second)
	}
}

func TestDirFingerprint_missingDir(t *testing.T) {
	if got := dirFingerprint(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Errorf("dirFingerprint(missing) = %q, want empty", got)
	}
}
