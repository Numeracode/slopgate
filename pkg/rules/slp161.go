package rules

import (
	"regexp"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP161 flags trailing whitespace on added lines. Trailing whitespace
// creates noisy diffs and can trip up some tooling (e.g. shell
// here-documents, Makefile continuations).
type SLP161 struct{}

func (SLP161) ID() string                { return "SLP161" }
func (SLP161) DefaultSeverity() Severity { return SeverityInfo }
func (SLP161) Description() string {
	return "trailing whitespace detected"
}

// slp161TrailingWS matches one or more whitespace characters at end of line.
var slp161TrailingWS = regexp.MustCompile(`\s+$`)

func (r SLP161) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}

		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				if ln.Content == "" {
					continue
				}

				if slp161TrailingWS.MatchString(ln.Content) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "trailing whitespace detected",
						Snippet:  ln.Content,
					})
				}
			}
		}
	}
	return out
}