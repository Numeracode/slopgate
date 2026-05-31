package rules

import (
	"fmt"
	"unicode/utf8"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP162 flags lines longer than 180 characters in source files. The
// threshold is raised from the former 140-char limit to reduce noise
// on legitimate generated or data-heavy lines while still catching
// truly excessive line lengths.
type SLP162 struct{}

func (SLP162) ID() string                { return "SLP162" }
func (SLP162) DefaultSeverity() Severity { return SeverityInfo }
func (SLP162) Description() string {
	return "line exceeds 180 characters"
}

const slp162MaxLineLen = 180

func (r SLP162) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		// Skip doc files (.md, .txt, etc.) and non-source files
		if isDocFile(f.Path) || !isSourceLikeFile(f.Path) {
			continue
		}

		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				if utf8.RuneCountInString(ln.Content) > slp162MaxLineLen {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  fmt.Sprintf("line is too long (%d chars, limit %d) - consider breaking into multiple lines", utf8.RuneCountInString(ln.Content), slp162MaxLineLen),
						Snippet:  ln.Content,
					})
				}
			}
		}
	}
	return out
}