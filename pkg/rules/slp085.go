package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP085 flags potential SQL injection via string concatenation in queries.
// This is a critical security issue that can lead to data breaches.
type SLP085 struct{}

func (SLP085) ID() string                { return "SLP085" }
func (SLP085) DefaultSeverity() Severity { return SeverityBlock }
func (SLP085) Description() string {
	return "SQL query built via string concatenation - use parameterized queries with placeholders"
}

var (
	// Common SQL DML keywords
	slp085SqlKeywords = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE"}
	// Matches template literal with SQL variables
	slp085TemplateLiteralPattern = regexp.MustCompile("`(?:SELECT|INSERT|UPDATE|DELETE|DROP|CREATE|ALTER|TRUNCATE)[^`]*\\${")
)

// Check scans for SQL queries built via string concatenation or template literals.
func (r SLP085) Check(d *diff.Diff) []Finding {
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

				content := strings.TrimSpace(ln.Content)

				// Skip if it's a comment
				if strings.HasPrefix(strings.TrimSpace(content), "//") || strings.HasPrefix(strings.TrimSpace(content), "#") || strings.HasPrefix(strings.TrimSpace(content), "--") {
					continue
				}

				// Check for template literal SQL
				if slp085TemplateLiteralPattern.MatchString(content) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "SQL built with template literal - use parameterized queries instead",
						Snippet:  content,
					})
					continue
				}

				// Check for string concatenation with SQL keywords
				upperContent := strings.ToUpper(content)
				hasSqlKeyword := false
				for _, kw := range slp085SqlKeywords {
					if strings.Contains(upperContent, kw) {
						hasSqlKeyword = true
						break
					}
				}

				if hasSqlKeyword {
					// Only treat a "+" as concatenation when it appears
					// outside of string/regex literals. A regex literal
					// like `\s+` contains a "+" that is not concatenation.
					codeOnly := stripCommentAndStrings(content)

					// Check for string concatenation operator
					if strings.Contains(codeOnly, "+") && (strings.Contains(content, "'") || strings.Contains(content, "\"")) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "SQL query built via string concatenation - use parameterized queries with $1, ? placeholders",
							Snippet:  content,
						})
						continue
					}

					// Check for backtick template
					if strings.Contains(content, "`") && strings.Contains(content, "${") {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "SQL built with template literal - use parameterized queries instead",
							Snippet:  content,
						})
					}
				}
			}
		}
	}
	return out
}
