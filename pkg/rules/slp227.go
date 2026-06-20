package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP227 flags repeated non-trivial string literals in a single added hunk.
// Repeating a string three or more times within one hunk is often a sign that
// a constant should be extracted.
//
// Trivial literals are excluded:
//   - "", "true", "false", "error", "%s", "%v", "%d", "%w", "%q", "ok",
//     "nil", "\n", " ", "-", ",", ".", "/", ":", "="
//   - single-character strings
//   - numeric-looking strings
//   - strings that are only punctuation/spaces
//
// The rule is diff-based: only added (`+`) lines are scanned.
//
// Severity: warn (heuristic).
type SLP227 struct{}

func (SLP227) ID() string                { return "SLP227" }
func (SLP227) DefaultSeverity() Severity { return SeverityWarn }
func (SLP227) Description() string {
	return "string literal repeated 3+ times in a hunk"
}

// stringLiteralRe matches double-quoted Go string literals (no escape support).
var stringLiteralRe = regexp.MustCompile(`"([^"\\]*)"`)

// trivialLiteralRe excludes known uninteresting literals and simple patterns.
var trivialLiteralRe = regexp.MustCompile(`^\s*$|^true$|^false$|^error$|^nil$|^ok$|^%[sdvwqtx]?$|^\d+$|^\W+$|^(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|TRACE|CONNECT)$`)

func (r SLP227) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) || isTestFile(f.Path) ||
			isGeneratedArtifactPath(f.Path) || isOpenAPIArtifactPath(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			var added []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					added = append(added, ln)
				}
			}
			if len(added) == 0 {
				continue
			}

			counts := map[string]int{}
			firstLine := map[string]int{}
			firstSnippet := map[string]string{}
			for _, ln := range added {
				content := ln.Content
				// Skip Go compiler directives and comments.
				trimmed := strings.TrimSpace(content)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				for _, m := range stringLiteralRe.FindAllStringSubmatch(content, -1) {
					lit := m[1]
					if trivial(lit) {
						continue
					}
					if _, ok := firstLine[lit]; !ok {
						firstLine[lit] = ln.NewLineNo
						firstSnippet[lit] = strings.TrimSpace(content)
					}
					counts[lit]++
				}
			}

			var keys []string
			for lit := range counts {
				if counts[lit] >= 3 {
					keys = append(keys, lit)
				}
			}
			sort.Strings(keys)
			for _, lit := range keys {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     firstLine[lit],
					Message:  "string literal \"" + lit + "\" repeated " + itoa(counts[lit]) + " times in this hunk",
					Snippet:  firstSnippet[lit],
				})
			}
		}
	}
	return out
}

func trivial(s string) bool {
	if len(s) <= 1 || len(s) < 6 {
		return true
	}
	return trivialLiteralRe.MatchString(s)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		i--
		n /= 10
	}
	return string(buf[i+1:])
}
