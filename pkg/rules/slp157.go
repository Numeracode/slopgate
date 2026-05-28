package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP157 detects the usage of parseInt() on request payload variables or query
// parameters without proper integer validation, which silently truncates float
// inputs (e.g., 4.5 becomes 4) instead of rejecting them.
type SLP157 struct{}

func (SLP157) ID() string                { return "SLP157" }
func (SLP157) DefaultSeverity() Severity { return SeverityWarn }
func (SLP157) Description() string {
	return "parseInt() on request payload variable without float validation may silently truncate float inputs"
}

var slp157ParseIntRe = regexp.MustCompile(`\bparseInt\s*\(\s*(?:req\.(?:body|query|params)\.[\w$]+|payload\.[\w$]+|body\.[\w$]+|query\.[\w$]+|params\.[\w$]+)\b`)
var slp157CommentLineRe = regexp.MustCompile(`^\s*(//|/\*|\*)`)
var slp157ValidationRe = regexp.MustCompile(`(?i)validate|schema|regex|match|test\b|zod|joi|yup|safeParse|^\d+$`)

func (r SLP157) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isJSOrTSFile(f.Path) {
			continue
		}

		for _, h := range f.Hunks {
			// Check if this hunk has any validation keywords
			hasValidation := false
			for _, ln := range h.Lines {
				if slp157ValidationRe.MatchString(ln.Content) {
					hasValidation = true
					break
				}
			}
			if hasValidation {
				continue
			}

			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				trimmed := strings.TrimSpace(ln.Content)
				if trimmed == "" || slp157CommentLineRe.MatchString(trimmed) {
					continue
				}

				if slp157ParseIntRe.MatchString(trimmed) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Message:  "parseInt() truncates float inputs silently. Validate parameter shape or use a schema validator to reject non-integers.",
						File:     f.Path,
						Line:     ln.NewLineNo,
					})
				}
			}
		}
	}
	return out
}
