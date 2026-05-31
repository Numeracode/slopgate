package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP160 flags TODO/FIXME/HACK/XXX comments that do not include a ticket
// reference (e.g. SLOP-123, CODE-456). Unticketed TODOs tend to linger
// indefinitely; requiring a reference makes them trackable and actionable.
type SLP160 struct{}

func (SLP160) ID() string                { return "SLP160" }
func (SLP160) DefaultSeverity() Severity { return SeverityInfo }
func (SLP160) Description() string {
	return "TODO/FIXME comment without ticket reference"
}

// slp160TodoPattern matches common task-marker keywords in comments.
// Word boundaries prevent false positives on todoList, hackathon, etc.
// XXX must be followed by a colon or parenthesized note to avoid matching
// arbitrary lowercase "xxx" strings in code.
var slp160TodoPattern = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK)\b|XXX[\s:(]`)

// slp160TicketRefPattern matches ticket references like SLOP-123 or CODE-456.
// Requires uppercase letter prefix to avoid matching lowercase strings like
// "fixme-123" which are not project ticket references.
var slp160TicketRefPattern = regexp.MustCompile(`\b[A-Z]{2,}-\d+\b`)

func (r SLP160) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if isTestFile(f.Path) {
			continue
		}
		if isDocFile(f.Path) {
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
				content := strings.TrimSpace(ln.Content)
				if content == "" {
					continue
				}

				if slp160TodoPattern.MatchString(content) {
					if !slp160TicketRefPattern.MatchString(content) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "TODO/FIXME comment without ticket reference - add ticket number",
							Snippet:  ln.Content,
						})
					}
				}
			}
		}
	}
	return out
}