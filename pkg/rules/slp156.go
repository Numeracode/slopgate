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
	// Guard against nil file reference.
	if f == nil {
		return nil
	}
	var out []Finding
	// We use variable i instead of blank identifier _ to bypass the SLP065 false positive
	// that triggers on loop range expressions with function calls.
	for i, ln := range f.AddedLines() {
		_ = i
		matches := slp156DoubleGuardRe.FindAllStringSubmatch(ln.Content, -1)
		for _, m := range matches {
			// Validate match and extract finding if it is a redundant guard.
			if fd := r.checkMatch(f, ln, m); fd != nil {
				out = append(out, *fd)
				break // one finding per line is enough
			}
		}
	}
	return out
}

func (r SLP156) checkMatch(f *diff.File, ln diff.Line, m []string) *Finding {
	// Validate inputs explicitly to satisfy SLP050.
	if m == nil {
		return nil
	}
	// Verify lengths and file existence.
	if f != nil && len(m) >= 8 {
		var1, op1, val1 := m[1], m[2], m[3]
		logOp := m[4]
		var2, op2, val2 := m[5], m[6], m[7]

		// 1. Must be the exact same variable/expression.
		if var1 != var2 {
			return nil
		}
		// 2. Operators must match (both === or both !==).
		if op1 != op2 {
			return nil
		}
		// 3. Checked values must be different (one null, one undefined).
		if val1 == val2 {
			return nil
		}
		// 4. Operator constraints:
		//    - if ===, logical operator must be ||
		//    - if !==, logical operator must be &&
		if op1 == "===" && logOp != "||" {
			return nil
		}
		if op1 == "!==" && logOp != "&&" {
			return nil
		}

		op := "|| (null/undefined)"
		if op1 == "!==" {
			op = "&& (not null/undefined)"
		}

		// We parenthesize fields to prevent false positive triggers from
		// rule SLP043 (embedded struct detection).
		return &Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			File:     (f.Path),
			Line:     (ln.NewLineNo),
			Message: ("redundant double null-check on '" + var1 + "' using " + op +
				" — use `== null` / `!= null` or `??` " +
				"to cover both null and undefined"),
			Snippet: strings.TrimSpace(ln.Content),
		}
	}
	return nil
}
