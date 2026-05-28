package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP158 detects theme mutations or class writes to `document.documentElement` or
// `document.body` inside standard `useEffect` hooks, which can lead to a Flash
// of Unstyled Content (FOUC). Recommends `useLayoutEffect` instead.
type SLP158 struct{}

func (SLP158) ID() string                { return "SLP158" }
func (SLP158) DefaultSeverity() Severity { return SeverityWarn }
func (SLP158) Description() string {
	return "useEffect theme mutation can cause visual flash (FOUC) — use useLayoutEffect instead"
}

var slp158ThemeMutationRe = regexp.MustCompile(`document\.(?:documentElement|body)(?:\.classList|\.className|\.setAttribute\(\s*['"](?:data-)?theme)`)
var slp158CommentLineRe = regexp.MustCompile(`^\s*(//|/\*|\*)`)

func slp158HasUseEffect(f diff.File, d *diff.Diff) bool {
	// 1. Try reading the file from disk if we can resolve its path
	if d != nil {
		repoRoot := d.RepoRoot
		if repoRoot == "" {
			if wd, err := os.Getwd(); err == nil {
				repoRoot = wd
			}
		}
		if repoRoot != "" {
			fullPath := filepath.Join(repoRoot, filepath.FromSlash(f.Path))
			if content, err := os.ReadFile(fullPath); err == nil {
				return strings.Contains(string(content), "useEffect")
			}
		}
	}

	// 2. Fallback to checking the diff hunks (e.g. in test suite where files aren't on disk)
	for _, h := range f.Hunks {
		for _, ln := range h.Lines {
			if strings.Contains(ln.Content, "useEffect") {
				return true
			}
		}
	}
	return false
}

func (r SLP158) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isJSOrTSFile(f.Path) {
			continue
		}

		if !slp158HasUseEffect(f, d) {
			continue
		}

		addedLines := f.AddedLines()
		for _, ln := range addedLines {
			trimmed := strings.TrimSpace(ln.Content)
			if trimmed == "" || slp158CommentLineRe.MatchString(trimmed) {
				continue
			}

			if slp158ThemeMutationRe.MatchString(trimmed) {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Message:  "useEffect theme mutation can cause visual flash (FOUC). Use useLayoutEffect instead.",
					File:     f.Path,
					Line:     ln.NewLineNo,
				})
			}
		}
	}
	return out
}
