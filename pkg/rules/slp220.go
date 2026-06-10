package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP220 flags a filepath.Walk or filepath.WalkDir call where the callback
// performs a long-running walk but doesn't check ctx.Err() inside.
// Whimsy PR #1961 reviewer flagged `treeSize()` that walked whole trees even
// after ctx.Done().
type SLP220 struct{}

func (SLP220) ID() string                { return "SLP220" }
func (SLP220) DefaultSeverity() Severity { return SeverityWarn }
func (SLP220) Description() string {
	return "filepath.Walk/WalkDir callback doesn't check ctx.Err() — walk not cancellable"
}

var walkCallRe = regexp.MustCompile(`\bfilepath\.(Walk|WalkDir)\s*\(`)
var ctxErrCheckRe = regexp.MustCompile(`\bctx\.Err\(|context\.Canceled|ctx\.Done\(\)`)

func (r SLP220) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, ln := range f.AddedLines() {
			if !walkCallRe.MatchString(ln.Content) {
				continue
			}
			// Look at the whole hunk for ctx.Err() usage.
			hunkHasCtx := false
			for _, h := range f.Hunks {
				for _, hln := range h.Lines {
					if hln.NewLineNo >= ln.NewLineNo-5 && hln.NewLineNo <= ln.NewLineNo+80 &&
						ctxErrCheckRe.MatchString(hln.Content) {
						hunkHasCtx = true
						break
					}
				}
				if hunkHasCtx {
					break
				}
			}
			if hunkHasCtx {
				continue
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     ln.NewLineNo,
				Message:  "filepath.Walk without ctx.Err() check — walk can't be cancelled and may run to completion",
				Snippet:  strings.TrimSpace(ln.Content),
			})
		}
	}
	return out
}
