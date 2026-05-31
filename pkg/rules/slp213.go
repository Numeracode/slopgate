package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP213 flags regex patterns that may unintentionally match empty strings
// due to `*` (zero-or-more) quantifiers on path segments or critical parts.
// Reviewers frequently flag patterns like `/path/*/` which match `/path/`
// (empty capture) when the intent was to require at least one character.
//
// This specifically targets regex patterns in string literals that use `*`
// at the end of a path-like segment or after a separator where `+` (one-or-more)
// would be more correct.
type SLP213 struct{}

func (SLP213) ID() string                { return "SLP213" }
func (SLP213) DefaultSeverity() Severity { return SeverityWarn }
func (SLP213) Description() string {
	return "regex with * quantifier may match empty string — consider using +"
}

// slp213RegexStringRe matches common regex pattern declarations.
var slp213RegexStringRe = regexp.MustCompile(`(?:new\s+RegExp|RegExp\(|/\^[^/]*\$/|Pattern\s*=\s*["'\x60])`)

// slp213StarAfterSep matches `*` that comes right after a separator or anchor
// where empty match is likely unintended.
var slp213StarAfterSep = regexp.MustCompile(`(?:/|\\\\)\*`)

func (r SLP213) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isSourceLikeFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				content := ln.Content
				// Only look at lines that contain regex-like patterns
				if !slp213RegexStringRe.MatchString(content) && !strings.Contains(content, "Pattern") {
					continue
				}
				if slp213StarAfterSep.MatchString(content) && !strings.Contains(content, ".*") {
					// Exclude cases where .* is used (intentional catch-all)
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "regex uses * (zero-or-more) after a separator — may match empty string; consider using + (one-or-more) if at least one character is required",
						Snippet:  content,
					})
				}
			}
		}
	}
	return out
}