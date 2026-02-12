package diff

import (
	"bufio"
	"regexp"
	"strings"
)

// BinaryMarker is the prefix git uses when a file is binary.
const binaryMarker = "Binary files "

// hunkHeader matches @@ -oldStart,oldCount +newStart,newCount @@ optional
var hunkHeaderRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)

// ParseUnifiedDiff parses the output of `git diff --no-color` and returns
// hunks. Binary file blocks (git emits "Binary files a/x b/x differ") are
// skipped; no hunks are produced for those files. Empty diff produces nil.
func ParseUnifiedDiff(diffOutput string) ([]Hunk, error) {
	if strings.TrimSpace(diffOutput) == "" {
		return nil, nil
	}

	sections := splitByFileSections(diffOutput)
	var hunks []Hunk
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		if strings.Contains(section, binaryMarker) {
			continue
		}
		filePath, hunkBlocks, err := parseFileSection(section)
		if err != nil {
			return nil, err
		}
		for _, block := range hunkBlocks {
			hunks = append(hunks, Hunk{
				FilePath:   filePath,
				RawContent: block,
				Context:    block,
			})
		}
	}
	return hunks, nil
}

// splitByFileSections splits diff output by "diff --git " so each section
// is one file's diff (or one binary notice).
func splitByFileSections(out string) []string {
	const prefix = "diff --git "
	var sections []string
	start := 0
	for {
		i := strings.Index(out[start:], prefix)
		if i < 0 {
			if start < len(out) && strings.TrimSpace(out[start:]) != "" {
				sections = append(sections, out[start:])
			}
			break
		}
		pos := start + i
		if pos > start && strings.TrimSpace(out[start:pos]) != "" {
			sections = append(sections, out[start:pos])
		}
		start = pos
		// find next "diff --git " or end
		next := strings.Index(out[start+len(prefix):], prefix)
		if next < 0 {
			sections = append(sections, out[start:])
			break
		}
		sections = append(sections, out[start:start+len(prefix)+next])
		start = start + len(prefix) + next
	}
	return sections
}

func parseFileSection(section string) (filePath string, hunkBlocks []string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(section))
	var (
		pathA, pathB string
		inHunk       bool
		currentLines []string
	)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git ") {
			pathA, pathB = parseDiffGitLine(line)
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			if pathB == "" && pathA == "" {
				pathA = parsePathLine(line, "--- ")
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			if pathB == "" {
				pathB = parsePathLine(line, "+++ ")
			}
			continue
		}
		if hunkHeaderRegex.MatchString(line) {
			if inHunk && len(currentLines) > 0 {
				hunkBlocks = append(hunkBlocks, strings.Join(currentLines, "\n"))
			}
			currentLines = []string{line}
			inHunk = true
			continue
		}
		if inHunk && (line == "" || line[0] == ' ' || line[0] == '-' || line[0] == '+') {
			currentLines = append(currentLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if inHunk && len(currentLines) > 0 {
		hunkBlocks = append(hunkBlocks, strings.Join(currentLines, "\n"))
	}
	filePath = pathB
	if filePath == "" {
		filePath = pathA
	}
	return filePath, hunkBlocks, nil
}

func parseDiffGitLine(line string) (a, b string) {
	// "diff --git a/path b/path"
	rest := strings.TrimPrefix(line, "diff --git ")
	parts := strings.Fields(rest)
	if len(parts) >= 2 {
		a = trimDiffPath(parts[0])
		b = trimDiffPath(parts[1])
	}
	return a, b
}

func trimDiffPath(s string) string {
	if len(s) >= 2 && (s[0] == 'a' || s[0] == 'b') && s[1] == '/' {
		return s[2:]
	}
	return s
}

func parsePathLine(line, prefix string) string {
	s := strings.TrimPrefix(line, prefix)
	// "/dev/null" or "a/path" or "b/path"
	if idx := strings.Index(s, "\t"); idx >= 0 {
		s = s[:idx]
	}
	return trimDiffPath(s)
}
