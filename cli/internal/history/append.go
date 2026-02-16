// Append and rotation for .review/history.jsonl.

package history

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"stet/cli/internal/erruser"
)

// ReadRecords reads all history records from stateDir: rotated archives (history.jsonl.N.gz)
// in ascending order by N, then the active history.jsonl. Returns concatenated records
// (oldest first). Missing or empty files are skipped. Used by optimizer and stet stats.
func ReadRecords(stateDir string) ([]Record, error) {
	path := filepath.Join(stateDir, historyFilename)
	// List and sort rotated archives by N (1, 2, 3, ...).
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil, erruser.New("Could not read history directory.", err)
	}
	type numbered struct {
		n    int
		path string
	}
	var archives []numbered
	prefix := historyGzPrefix
	suffix := historyGzSuffix
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "." || name == ".." || strings.Contains(name, string(filepath.Separator)) {
			continue
		}
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		mid := name[len(prefix) : len(name)-len(suffix)]
		n, err := strconv.Atoi(mid)
		if err != nil || n < 1 {
			continue
		}
		archives = append(archives, numbered{n: n, path: filepath.Join(stateDir, name)})
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].n < archives[j].n })

	var out []Record
	for _, a := range archives {
		recs, err := readRecordsFromGzip(a.path)
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	// Active file last (most recent).
	plainRecs, err := readRecordsFromFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, erruser.New("Could not read history file.", err)
	}
	out = append(out, plainRecs...)
	return out, nil
}

func readRecordsFromGzip(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()
	lines, err := readLinesFromReader(gr)
	if err != nil {
		return nil, err
	}
	return parseRecordLines(lines)
}

func readRecordsFromFile(path string) ([]Record, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	return parseRecordLines(lines)
}

func parseRecordLines(lines []string) ([]Record, error) {
	var out []Record
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("invalid history line: %w", err)
		}
		out = append(out, rec)
	}
	return out, nil
}

const (
	historyFilename      = "history.jsonl"
	historyGzPrefix      = "history.jsonl."
	historyGzSuffix      = ".gz"
	DefaultMaxRecords    = 1000
	maxRotatedArchives   = 5
)

// Append writes one record as a single JSON line to stateDir/history.jsonl.
// Creates stateDir and the file if missing. If maxRecords > 0 and the file
// has more than maxRecords lines after appending, the file is rotated to
// keep only the last maxRecords lines (atomic write via temp + rename).
func Append(stateDir string, record Record, maxRecords int) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return erruser.New("Could not create state directory for history.", err)
	}
	path := filepath.Join(stateDir, historyFilename)
	line, err := json.Marshal(record)
	if err != nil {
		return erruser.New("Could not record review history.", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return erruser.New("Could not record review history.", err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return erruser.New("Could not record review history.", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		_ = f.Close()
		return erruser.New("Could not record review history.", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return erruser.New("Could not record review history.", err)
	}
	if err := f.Close(); err != nil {
		return erruser.New("Could not record review history.", err)
	}

	if maxRecords > 0 {
		if err := rotateIfNeeded(path, maxRecords); err != nil {
			return err
		}
	}
	return nil
}

// rotateIfNeeded reads path, and if it has more than maxRecords lines,
// writes the dropped lines to a gzipped archive (history.jsonl.N.gz), then
// rewrites the active file with only the last maxRecords lines (atomic: temp + rename).
// Keeps at most maxRotatedArchives .gz files; oldest is removed when exceeding.
func rotateIfNeeded(path string, maxRecords int) error {
	lines, err := readLines(path)
	if err != nil {
		return erruser.New("Could not read history for rotation.", err)
	}
	if len(lines) <= maxRecords {
		return nil
	}
	dropped := lines[:len(lines)-maxRecords]
	keep := lines[len(lines)-maxRecords:]
	dir := filepath.Dir(path)

	// Write dropped lines to a new .gz archive.
	nextNum, err := nextRotatedArchiveNumber(dir)
	if err != nil {
		return err
	}
	archivePath := filepath.Join(dir, historyGzPrefix+strconv.Itoa(nextNum)+historyGzSuffix)
	if err := writeGzippedLines(archivePath, dropped); err != nil {
		return erruser.New("Could not write rotated history archive.", err)
	}
	// Remove oldest archives if over cap.
	if err := pruneRotatedArchives(dir); err != nil {
		return err
	}

	// Rewrite active file with keep (atomic).
	f, err := os.CreateTemp(dir, "history.*.tmp")
	if err != nil {
		return erruser.New("Could not rotate history file.", err)
	}
	tmpPath := f.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	for _, l := range keep {
		if _, err := f.WriteString(l); err != nil {
			_ = f.Close()
			return erruser.New("Could not rotate history file.", err)
		}
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return erruser.New("Could not rotate history file.", err)
	}
	if err := f.Close(); err != nil {
		return erruser.New("Could not rotate history file.", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return erruser.New("Could not rotate history file.", err)
	}
	return nil
}

// nextRotatedArchiveNumber returns the next sequence number for history.jsonl.N.gz (1-based).
func nextRotatedArchiveNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	prefix := historyGzPrefix
	suffix := historyGzSuffix
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		mid := e.Name()[len(prefix) : len(e.Name())-len(suffix)]
		n, err := strconv.Atoi(mid)
		if err != nil || n < 1 {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

func writeGzippedLines(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gw := gzip.NewWriter(f)
	for _, l := range lines {
		if _, err := gw.Write([]byte(l)); err != nil {
			_ = gw.Close()
			return err
		}
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return f.Sync()
}

// pruneRotatedArchives removes oldest history.jsonl.N.gz files when count exceeds maxRotatedArchives.
// Archives are ordered by numeric N (ascending) so the oldest (lowest N) is removed first.
func pruneRotatedArchives(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type numberedArchive struct {
		n    int
		name string
	}
	var archives []numberedArchive
	prefix := historyGzPrefix
	suffix := historyGzSuffix
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		mid := e.Name()[len(prefix) : len(e.Name())-len(suffix)]
		n, err := strconv.Atoi(mid)
		if err != nil || n < 1 {
			continue
		}
		archives = append(archives, numberedArchive{n: n, name: e.Name()})
	}
	if len(archives) <= maxRotatedArchives {
		return nil
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].n < archives[j].n })
	for i := 0; i < len(archives)-maxRotatedArchives; i++ {
		if err := os.Remove(filepath.Join(dir, archives[i].name)); err != nil {
			return err
		}
	}
	return nil
}

// maxLineSize is the maximum size of a single history line. bufio.Scanner defaults to 64KB;
// history records can include full review output (findings, prompts), so allow up to 4MB.
const maxLineSize = 4 * 1024 * 1024

// readLines loads the entire file into memory. Very large history files may consume
// significant memory. maxLineSize limits single-line size; lines larger than that are rejected.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLinesFromReader(f)
}

func readLinesFromReader(r io.Reader) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLineSize)
	for sc.Scan() {
		lines = append(lines, sc.Text()+"\n")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
