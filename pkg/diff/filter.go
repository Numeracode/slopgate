package diff

import (
	"bufio"
	"io"
	"path/filepath"
	"strings"
)

// FilterIgnored returns a copy of d with any File whose path matches
// one of the glob patterns removed. Patterns follow the syntax of
// filepath.Match, with one extension: a leading "**/" matches any
// number of leading path segments — so "**/foo_test.go" matches
// "pkg/rules/foo_test.go" and "foo_test.go" alike.
//
// If patterns is empty or nil, the input Diff is returned unchanged.
func FilterIgnored(d *Diff, patterns []string) *Diff {
	if len(patterns) == 0 || d == nil {
		return d
	}
	out := &Diff{
		Files:            make([]File, 0, len(d.Files)),
		IgnoredFiles:     append([]File(nil), d.IgnoredFiles...),
		RepoRoot:         d.RepoRoot,
		Staged:           d.Staged,
		SnapshotRef:      d.SnapshotRef,
		SnapshotWorktree: d.SnapshotWorktree,
	}
	for _, f := range d.Files {
		if matchesAny(f.Path, patterns) {
			out.IgnoredFiles = append(out.IgnoredFiles, f)
			continue
		}
		out.Files = append(out.Files, f)
	}
	return out
}

// matchesAny reports whether path matches any of the given patterns.
func matchesAny(path string, patterns []string) bool {
	for _, p := range patterns {
		if matchesPattern(path, p) {
			return true
		}
	}
	return false
}

// matchesPattern implements a small superset of filepath.Match:
//   - "**/<tail>" matches <tail> at any directory depth, including zero.
//
// Anything else is passed through to filepath.Match as-is.
func matchesPattern(path, pattern string) bool {
	if strings.HasPrefix(pattern, "**/") {
		tail := strings.TrimPrefix(pattern, "**/")
		// Zero leading segments.
		if ok, _ := filepath.Match(tail, path); ok {
			return true
		}
		// One or more leading segments.
		for i := 0; i < len(path); i++ {
			if path[i] == '/' {
				if ok, _ := filepath.Match(tail, path[i+1:]); ok {
					return true
				}
			}
		}
		return false
	}
	ok, _ := filepath.Match(pattern, path)
	return ok
}

// ParseIgnoreFile reads a .slopgateignore file and returns the list of
// glob patterns. Blank lines and lines starting with '#' are ignored.
// Trailing whitespace is trimmed.
func ParseIgnoreFile(r io.Reader) ([]string, error) {
	var out []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
