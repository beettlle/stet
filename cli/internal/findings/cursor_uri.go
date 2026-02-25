package findings

import (
	"path/filepath"
	"strconv"
)

// SetCursorURIs sets CursorURI on each finding in list when empty, using
// repoRoot (absolute or relative to cwd) + finding File and Line (or Range).
// Builds file:// URIs so the extension can open at location. If building the
// path fails for a finding, that finding's CursorURI is left unchanged.
func SetCursorURIs(repoRoot string, list []Finding) {
	for i := range list {
		if list[i].CursorURI != "" {
			continue
		}
		absPath, err := filepath.Abs(filepath.Join(repoRoot, list[i].File))
		if err != nil {
			continue
		}
		uri := "file://" + filepath.ToSlash(absPath)
		line := list[i].Line
		if list[i].Range != nil && list[i].Range.Start > 0 {
			line = list[i].Range.Start
		}
		if line > 0 {
			if list[i].Range != nil && list[i].Range.Start > 0 && list[i].Range.End >= list[i].Range.Start {
				uri += "#L" + strconv.Itoa(list[i].Range.Start) + "-" + strconv.Itoa(list[i].Range.End)
			} else {
				uri += "#L" + strconv.Itoa(line)
			}
		}
		list[i].CursorURI = uri
	}
}
