// Append and rotation for .review/history.jsonl.

package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"stet/cli/internal/erruser"
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
// rewrites it with only the last maxRecords lines (atomic: temp + rename).
func rotateIfNeeded(path string, maxRecords int) error {
	lines, err := readLines(path)
	if err != nil {
		return erruser.New("Could not read history for rotation.", err)
	}
	if len(lines) <= maxRecords {
		return nil
	}
	keep := lines[len(lines)-maxRecords:]
	dir := filepath.Dir(path)
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
	var lines []string
	sc := bufio.NewScanner(f)
	// Start with 64KB buffer; Buffer allows growth up to maxLineSize for large lines.
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
