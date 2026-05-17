package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP056 flags hardcoded secrets in added lines across any file type.
type SLP056 struct{}

func (SLP056) ID() string                { return "SLP056" }
func (SLP056) DefaultSeverity() Severity { return SeverityBlock }
func (SLP056) Description() string {
	return "hardcoded secrets detected in added lines"
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["']\w+`),
	regexp.MustCompile(`(?i)password\s*[:=]\s*["'][^"']+`),
	regexp.MustCompile(`(?i)secret\s*[:=]\s*["']\w+`),
	regexp.MustCompile(`(?i)token\s*[:=]\s*["']\w+`),
	regexp.MustCompile(`(?i)bearer\s+\w+`),
	regexp.MustCompile(`(?i)aws_access_key_id\s*[:=]\s*(?:"[^"]+"|'[^']+'|[A-Z0-9]{16,})`),
}

// skipWordsLower contains words that indicate example/test data, lowercased.
// Checked against tokens split on non-alphanumeric boundaries to avoid substring matches.
var skipWordsLower = map[string]bool{
	"example": true, "sample": true, "dummy": true, "test": true,
	"placeholder": true, "fake": true, "mock": true, "todo": true, "fixme": true,
}

// tokenSplitRe splits a string into tokens on non-alphanumeric boundaries.
var tokenSplitRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func hasSkipWordToken(content string) bool {
	lower := strings.ToLower(content)
	tokens := tokenSplitRe.Split(lower, -1)
	for _, tok := range tokens {
		if skipWordsLower[tok] {
			return true
		}
	}
	return false
}

func slp056CommentOnlyLine(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "--")
}

func slp056StripInlineComment(content string) string {
	for {
		start := strings.Index(content, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(content[start+2:], "*/")
		if end < 0 {
			content = content[:start]
			break
		}
		content = content[:start] + content[start+2+end+2:]
	}
	for _, marker := range []string{"//", "#", "--"} {
		if idx := strings.Index(content, marker); idx >= 0 {
			return content[:idx]
		}
	}
	return content
}

func slp056ShouldSkip(content string) bool {
	if slp056CommentOnlyLine(content) {
		return hasSkipWordToken(content)
	}
	return hasSkipWordToken(slp056StripInlineComment(content))
}

func slp056MatchesPrivateKey(content string) bool {
	re := regexp.MustCompile(`(?i)\bprivate_key\b\s*[:=]\s*(.*)$`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return false
	}
	rhs := strings.TrimSpace(m[1])
	if rhs == "" || strings.HasPrefix(rhs, `"`) || strings.HasPrefix(rhs, `'`) ||
		strings.HasPrefix(rhs, "`") || strings.HasPrefix(strings.ToUpper(rhs), "-----BEGIN") {
		return true
	}
	lower := strings.ToLower(rhs)
	if strings.Contains(lower, "getenv") || strings.Contains(lower, "os.environ") ||
		strings.Contains(lower, "read_file") || strings.Contains(rhs, "(") || strings.Contains(rhs, ".") {
		return false
	}
	return true
}

// Check scans added lines for hardcoded secrets while skipping generated OpenAPI artifacts.
func (r SLP056) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isOpenAPIArtifactPath(f.Path) {
			continue
		}
		for _, ln := range f.AddedLines() {
			if slp056ShouldSkip(ln.Content) {
				continue
			}
			matched := slp056MatchesPrivateKey(ln.Content)
			if !matched {
				for _, re := range secretPatterns {
					if re.MatchString(ln.Content) {
						matched = true
						break
					}
				}
			}
			if matched {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "hardcoded secret pattern detected — use environment variables or a secret manager",
					Snippet:  "[REDACTED]",
				})
			}
		}
	}
	return out
}
