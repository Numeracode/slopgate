package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP035 flags console and debugger statements left in production code.
//
// Pattern: Console statements and debugger statements.
//
// Rationale: Console/debugger statements are debug artifacts that should
// not ship to production and can leak sensitive information.
type SLP035 struct{}

func (SLP035) ID() string                { return "SLP035" }
func (SLP035) DefaultSeverity() Severity { return SeverityInfo }
func (SLP035) Description() string {
	return "console or debugger statement detected in code"
}

// Named regex patterns for efficient lookup
var consolePattern = regexp.MustCompile(`(?i)console\.(log|debug|info|warn|error)\s*\(`)
var debuggerPattern = regexp.MustCompile(`(?i)\bdebugger\b`)

func (r SLP035) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		// Skip test files — console/debugger are often intentional in tests
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

				// Skip truly empty lines (no characters at all)
				if ln.Content == "" {
					continue
				}

				content := strings.TrimSpace(ln.Content)

				// Skip whitespace-only lines
				if content == "" {
					continue
				}

				// Check for console.log statements
				if consolePattern.MatchString(content) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "console statement detected in code - remove before production",
						Snippet:  ln.Content,
					})
				}

				// Check for debugger statements
				if debuggerPattern.MatchString(content) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "debugger statement detected in code - remove before production",
						Snippet:  ln.Content,
					})
				}
			}
		}
	}
	return out
}
