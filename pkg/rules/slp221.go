package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP221 flags exec.Command / exec.CommandContext calls followed by
// .Output(), .Run(), or .CombinedOutput() where the hunk never wires up
// Stderr. On failure the exit-error body discards whatever the child
// printed to stderr, so error logs are opaque.
//
// Reviewer pattern (whimsy PR #1952 snapnative.go:105): sourcery-ai noted
// "Capturing and appending command stderr would help diagnose why
// the subprocess failed." Sourcery flagged Output() returning just
// a generic exit error.
type SLP221 struct{}

func (SLP221) ID() string                { return "SLP221" }
func (SLP221) DefaultSeverity() Severity { return SeverityWarn }
func (SLP221) Description() string {
	return "exec.Command without Stderr/StderrPipe — failed subprocess stderr is lost"
}

var execCommandContextRe = regexp.MustCompile(`\bexec\.Command(Context)?\s*\(`)
var execRunRe = regexp.MustCompile(`\.(Output|Run|CombinedOutput)\s*\(\s*\)`)
var stderrWireRe = regexp.MustCompile(`\b(Stderr\s*=|StderrPipe|cmd\.Stderr|\.Stderr\s*=)`)

func (r SLP221) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			var execLine *diff.Line
			var runLine *diff.Line
			hunkText := ""
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					c := ln.Content
					hunkText += c + "\n"
					if execCommandContextRe.MatchString(c) {
						cp := ln
						execLine = &cp
					}
					if execRunRe.MatchString(c) {
						cp := ln
						runLine = &cp
					}
				}
			}
			if execLine == nil || runLine == nil {
				continue
			}
			if stderrWireRe.MatchString(hunkText) {
				continue
			}
			// Prefer the run-line position: that's the actual lossy site.
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     runLine.NewLineNo,
				Message:  "exec.Command .Output/.Run without Stderr/StderrPipe — subprocess failure stderr is discarded",
				Snippet:  strings.TrimSpace(runLine.Content),
			})
		}
	}
	return out
}
