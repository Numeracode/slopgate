// Package diff parses unified git diffs into a structured form that
// rules can query. Only the subset of the format that git actually
// emits is supported: standard headers, hunk headers, add/delete/context
// lines, new-file and deleted-file markers.
package diff

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// unquoteGitPath attempts to unquote a git C-style quoted path.
// Git quotes paths containing spaces, non-ASCII, or special characters.
// If the input is not quoted, it is returned unchanged.
func unquoteGitPath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if unquoted, err := strconv.Unquote(p); err == nil {
			return unquoted
		}
	}
	return p
}

// LineKind tags each line within a hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDelete
)

// Diff is the top-level parsed representation of a unified diff.
type Diff struct {
	Files            []File
	ignoredFiles     []File // files filtered from Files by ignore patterns but retained for meta-rules
	RepoRoot         string // optional absolute worktree root for rules that inspect files
	Staged           bool   // true when the diff came from git diff --cached
	SnapshotRef      string // optional git snapshot source for the new side, e.g. HEAD or :
	SnapshotWorktree bool   // true when the new-side snapshot matches the live worktree
}

// File represents a single file's changes within a diff.
type File struct {
	Path     string // new path (or old path if deleted)
	OldPath  string // path in the old revision
	IsNew    bool
	IsDelete bool
	Hunks    []Hunk
}

// Hunk is a single @@ ... @@ block.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []Line
}

// Line is one unified-diff line within a hunk.
type Line struct {
	Kind      LineKind
	Content   string // text of the line, without the leading +/- space
	NewLineNo int    // line number in the new file (0 for Delete)
	OldLineNo int    // line number in the old file (0 for Add)
}

// AllFiles returns both active and ignored files so meta-rules can reason about
// companion test changes without re-exposing ignored files to normal scanners.
func (d *Diff) AllFiles() []File {
	if d == nil {
		return nil
	}
	out := make([]File, 0, len(d.Files)+len(d.ignoredFiles))
	out = append(out, d.Files...)
	out = append(out, d.ignoredFiles...)
	return out
}

// AddedLines returns just the added lines across all hunks of a file, in order.
func (f File) AddedLines() []Line {
	var out []Line
	for _, h := range f.Hunks {
		for _, ln := range h.Lines {
			if ln.Kind == LineAdd {
				out = append(out, ln)
			}
		}
	}
	return out
}

// DeletedLines returns just the deleted lines across all hunks of a file, in order.
func (f File) DeletedLines() []Line {
	var out []Line
	for _, h := range f.Hunks {
		for _, ln := range h.Lines {
			if ln.Kind == LineDelete {
				out = append(out, ln)
			}
		}
	}
	return out
}

// Parse reads a unified diff from r and returns a Diff.
// It is forgiving: unknown header lines are ignored. An empty input
// yields an empty Diff with no error.
func Parse(r io.Reader) (*Diff, error) {
	d := &Diff{}
	scanner := bufio.NewScanner(r)
	// Allow very large lines (up to 1 MiB) — AI-generated diffs sometimes
	// inline huge strings on a single line.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var cur *File
	var hunk *Hunk
	var newLineNo, oldLineNo int

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "diff --git "):
			// Flush any prior hunk/file.
			if hunk != nil {
				cur.Hunks = append(cur.Hunks, *hunk)
				hunk = nil
			}
			if cur != nil {
				d.Files = append(d.Files, *cur)
			}
			cur = &File{}
			// Default paths from the git diff header; they may be
			// overwritten by the --- / +++ lines that follow.
			if a, b, ok := parseGitDiffPaths(line); ok {
				cur.OldPath = a
				cur.Path = b
			}

		case strings.HasPrefix(line, "new file mode"):
			if cur != nil {
				cur.IsNew = true
			}

		case strings.HasPrefix(line, "deleted file mode"):
			if cur != nil {
				cur.IsDelete = true
			}

		case strings.HasPrefix(line, "--- "):
			if cur == nil {
				cur = &File{}
			}
			p := strings.TrimPrefix(line, "--- ")
			if p == "/dev/null" {
				cur.IsNew = true
				cur.OldPath = ""
			} else {
				cur.OldPath = stripPathPrefix(p)
			}

		case strings.HasPrefix(line, "+++ "):
			if cur == nil {
				cur = &File{}
			}
			p := strings.TrimPrefix(line, "+++ ")
			if p == "/dev/null" {
				cur.IsDelete = true
				// Keep Path as the old path so rules always have a non-empty path.
				if cur.Path == "" {
					cur.Path = cur.OldPath
				}
			} else {
				cur.Path = stripPathPrefix(p)
			}

		case strings.HasPrefix(line, "@@"):
			if cur == nil {
				return nil, fmt.Errorf("hunk header with no file: %q", line)
			}
			if hunk != nil {
				cur.Hunks = append(cur.Hunks, *hunk)
			}
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			hunk = &h
			newLineNo = hunk.NewStart
			oldLineNo = hunk.OldStart

		default:
			if hunk == nil {
				// Header or metadata line we don't care about.
				continue
			}
			if len(line) == 0 {
				// An empty line inside a hunk counts as context with
				// an empty body.
				hunk.Lines = append(hunk.Lines, Line{
					Kind:      LineContext,
					Content:   "",
					NewLineNo: newLineNo,
					OldLineNo: oldLineNo,
				})
				newLineNo++
				oldLineNo++
				continue
			}
			prefix := line[0]
			body := line[1:]
			switch prefix {
			case '+':
				hunk.Lines = append(hunk.Lines, Line{
					Kind:      LineAdd,
					Content:   body,
					NewLineNo: newLineNo,
					OldLineNo: 0,
				})
				newLineNo++
			case '-':
				hunk.Lines = append(hunk.Lines, Line{
					Kind:      LineDelete,
					Content:   body,
					NewLineNo: 0,
					OldLineNo: oldLineNo,
				})
				oldLineNo++
			case ' ':
				hunk.Lines = append(hunk.Lines, Line{
					Kind:      LineContext,
					Content:   body,
					NewLineNo: newLineNo,
					OldLineNo: oldLineNo,
				})
				newLineNo++
				oldLineNo++
			case '\\':
				// "\ No newline at end of file" — ignore.
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if hunk != nil && cur != nil {
		cur.Hunks = append(cur.Hunks, *hunk)
	}
	if cur != nil {
		d.Files = append(d.Files, *cur)
	}
	return d, nil
}

// parseGitDiffPaths parses `diff --git a/foo b/bar` into ("foo", "bar").
func parseGitDiffPaths(line string) (string, string, bool) {
	// Format: diff --git a/<path> b/<path>
	// Paths may contain spaces, but git escapes those cases with quotes;
	// we handle only the common unquoted case.
	rest := strings.TrimPrefix(line, "diff --git ")
	// Find " b/" boundary from the end.
	idx := strings.LastIndex(rest, " b/")
	if idx < 0 {
		return "", "", false
	}
	a := strings.TrimPrefix(rest[:idx], "a/")
	b := strings.TrimPrefix(rest[idx+1:], "b/")
	return a, b, true
}

// stripPathPrefix removes the leading a/ or b/ that git puts on
// --- and +++ lines. It also drops any trailing timestamp that may
// appear on non-git diffs.
func stripPathPrefix(p string) string {
	p = unquoteGitPath(p)
	// Some non-git diffs have a tab-separated timestamp after the path.
	if tab := strings.IndexByte(p, '\t'); tab >= 0 {
		p = p[:tab]
	}
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return p
}

// parseHunkHeader parses "@@ -oldStart,oldLines +newStart,newLines @@ ..."
// where the ,lines part is optional and defaults to 1.
func parseHunkHeader(line string) (Hunk, error) {
	// Trim leading "@@ " and take up to the next " @@".
	s := strings.TrimPrefix(line, "@@")
	s = strings.TrimLeft(s, " ")
	end := strings.Index(s, "@@")
	if end < 0 {
		return Hunk{}, fmt.Errorf("malformed hunk header: %q", line)
	}
	s = strings.TrimSpace(s[:end])

	parts := strings.Fields(s)
	if len(parts) < 2 {
		return Hunk{}, fmt.Errorf("malformed hunk header: %q", line)
	}
	oldStart, oldLines, err := parseRangePart(parts[0], '-')
	if err != nil {
		return Hunk{}, err
	}
	newStart, newLines, err := parseRangePart(parts[1], '+')
	if err != nil {
		return Hunk{}, err
	}
	return Hunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

func parseRangePart(p string, sign byte) (int, int, error) {
	if len(p) == 0 || p[0] != sign {
		return 0, 0, fmt.Errorf("range part missing sign %q: %q", sign, p)
	}
	body := p[1:]
	var startStr, linesStr string
	if comma := strings.IndexByte(body, ','); comma >= 0 {
		startStr = body[:comma]
		linesStr = body[comma+1:]
	} else {
		startStr = body
		linesStr = "1"
	}
	start, err := strconv.Atoi(startStr)
	if err != nil {
		return 0, 0, fmt.Errorf("range start not int: %q", p)
	}
	lines, err := strconv.Atoi(linesStr)
	if err != nil {
		return 0, 0, fmt.Errorf("range lines not int: %q", p)
	}
	return start, lines, nil
}
