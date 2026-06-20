//go:build ignore_retired

package rules

import (
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP043 flags response structs with duplicate key fields.
//
// Rationale: When composing responses from embedded structs or adding duplicate fields,
// AI agents may accidentally create response shapes with duplicate keys (e.g., both
// embedded and explicit fields for the same data).
//
// Languages: Go.
//
// Scope: only added lines in Go files.
type SLP043 struct{}

func (SLP043) ID() string                { return "SLP043" }
func (SLP043) DefaultSeverity() Severity { return SeverityWarn }
func (SLP043) Description() string {
	return "response struct may have duplicate JSON keys"
}

func (r SLP043) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if !isGoFile(f.Path) {
			continue
		}

		// Detect embedded struct fields (type name on its own line inside a struct)
		// that lack an explicit json tag override, which can create duplicate keys.
		for _, line := range f.AddedLines() {
			content := strings.TrimSpace(line.Content)
			// Match embedded type: a line that is just a type name (possibly with *)
			// e.g., "  SomeType" or "  *SomeType" inside a struct, without a json tag.
			if (strings.HasPrefix(content, "*") || (len(content) > 0 && isUpperCaseLetter(content[0]))) &&
				!strings.Contains(content, "`") &&
				!strings.HasPrefix(content, "//") &&
				!strings.HasPrefix(content, "func") &&
				!strings.HasPrefix(content, "type ") &&
				!strings.HasPrefix(content, "package ") &&
				!strings.Contains(content, ":=") &&
				!strings.Contains(content, "(") &&
				!strings.HasPrefix(content, "}") &&
				strings.Contains(content, " ") {
				// This looks like an embedded type field without json tag
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     line.NewLineNo,
					Message:  r.Description(),
					Snippet:  content,
				})
			}
		}
	}
	return out
}

func isUpperCaseLetter(b byte) bool {
	return b >= 'A' && b <= 'Z'
}
