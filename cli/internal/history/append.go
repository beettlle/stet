// Append and rotation for .review/history.jsonl.

package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	historyFilename   = "history.jsonl"
	DefaultMaxRecords = 1000
)

// Append writes one record as a single JSON line to stateDir/history.jsonl.
// Creates stateDir and the file if missing. If maxRecords > 0 and the file
// has more than maxRecords lines after appending, the file is rotated to
// keep only the last maxRecords lines (atomic write via temp + rename).
func Append(stateDir string, record Record, maxRecords int) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("history: create state dir: %w", err)
	}
	path := filepath.Join(stateDir, historyFilename)
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("history: marshal record: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("history: open %s: %w", path, err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("history: write: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("history: write newline: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("history: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("history: close: %w", err)
	}

	if maxRecords > 0 {
		if err := rotateIfNeeded(path, maxRecords); err != nil {
			return err
		}
	}
	return nil
}

// rotateIfNeeded reads path, and if it has more than maxRecords lines,
// rewrites it with only the last maxRecords lines (atomic: temp + rename).
func rotateIfNeeded(path string, maxRecords int) error {
	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("history: read for rotation: %w", err)
	}
	if len(lines) <= maxRecords {
		return nil
	}
	keep := lines[len(lines)-maxRecords:]
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "history.*.tmp")
	if err != nil {
		return fmt.Errorf("history: create temp: %w", err)
	}
	tmpPath := f.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	for _, l := range keep {
		if _, err := f.WriteString(l); err != nil {
			_ = f.Close()
			return fmt.Errorf("history: write temp: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("history: sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("history: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("history: rename to %s: %w", path, err)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text()+"\n")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
