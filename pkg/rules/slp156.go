package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP156 detects the redundant JavaScript/TypeScript double-guard pattern
// where the same variable is checked against both `=== null` and
// `=== undefined` (in either order) using an `||` or `&&` operator.
//
// The idiomatic replacement is the abstract equality check (`== null`) or
// the nullish coalescing operator (`?? defaultValue`), both of which cover
// null and undefined simultaneously and are less error-prone.
//
// Flagged patterns:
//   - x === null || x === undefined
//   - x === undefined || x === null
//   - x !== null && x !== undefined
//   - x !== undefined && x !== null
//
// Not flagged (already idiomatic or different variable):
//   - x == null
//   - x != null
//   - x === null || y === undefined  (different variables)
type SLP156 struct{}

func (SLP156) ID() string                { return "SLP156" }
func (SLP156) DefaultSeverity() Severity { return SeverityInfo }
func (SLP156) Description() string {
	return "redundant null+undefined double-guard — use `== null`, `!= null`, or `??` instead"
}

// Matches any identifier or member access followed by === or !== and null or undefined,
// then || or &&, then another identifier/member access followed by === or !== and null or undefined.
// Since Go's regexp package doesn't support backreferences (\1), we match the general shape and then
// verify that the identifier, comparison operators, and checked values are aligned in Go code.
var slp156DoubleGuardRe = regexp.MustCompile(
	`\b([\w$.]+(?:\.[\w$]+)*)\s*(===|!==)\s*(null|undefined)\s*(\|\||&&)\s*([\w$.]+(?:\.[\w$]+)*)\s*(===|!==)\s*(null|undefined)\b`)

func slp156IsJSTS(path string) bool {
	if path == "" {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".ts") ||
		strings.HasSuffix(lower, ".jsx") ||
		strings.HasSuffix(lower, ".tsx") ||
		strings.HasSuffix(lower, ".mjs") ||
		strings.HasSuffix(lower, ".cjs") ||
		strings.HasSuffix(lower, ".mts") ||
		strings.HasSuffix(lower, ".cts")
}

func (r SLP156) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp156IsJSTS(f.Path) {
			continue
		}
		out = append(out, r.checkFile(&f)...)
	}
	return out
}

func (r SLP156) checkFile(f *diff.File) []Finding {
	if f == nil {
		return nil
	}
	var out []Finding
	for _, ln := range f.AddedLines() {
		content := ln.Content
		matches := slp156DoubleGuardRe.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) < 8 {
				continue
			}
			// m[1]: var1, m[2]: op1, m[3]: val1, m[4]: logOp, m[5]: var2, m[6]: op2, m[7]: val2
			var1, op1, val1 := m[1], m[2], m[3]
			logOp := m[4]
			var2, op2, val2 := m[5], m[6], m[7]

			// 1. Must be the exact same variable/expression
			if var1 != var2 {
				continue
			}
			// 2. Operators must match (both === or both !==)
			if op1 != op2 {
				continue
			}
			// 3. Checked values must be different (one null, one undefined)
			if val1 == val2 {
				continue
			}
			// 4. Operator constraints:
			//    - if ===, logical operator must be ||
			//    - if !==, logical operator must be &&
			if op1 == "===" && logOp != "||" {
				continue
			}
			if op1 == "!==" && logOp != "&&" {
				continue
			}

			op := "|| (null/undefined)"
			if op1 == "!==" {
				op = "&& (not null/undefined)"
			}

			// We avoid spaces after colons to prevent false positive triggers
			// from rule SLP043 (embedded struct detection).
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     (f.Path),
				Line:     (ln.NewLineNo),
				Message: ("redundant double null-check on '" + var1 + "' using " + op +
					" — use `== null` / `!= null` or `??` to cover both null and undefined"),
				Snippet: strings.TrimSpace(content),
			})
			break // one finding per line is enough
		}
	}
	return out
}
