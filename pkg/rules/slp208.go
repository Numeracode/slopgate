package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP208 detects TypeScript/JavaScript function declarations where a
// parameter with a default value appears before a required parameter.
//
// In JS/TS, default parameters must come last — otherwise calling the
// function with positional args becomes ambiguous and TypeScript raises
// a compile error. This rule catches the pattern in added diff lines.
//
// Flagged patterns:
//   - function foo(a = 1, b) { ... }
//   - const foo = (a = 1, b) => { ... }
//   - function foo(a, b = 1, c) { ... }
//
// Not flagged:
//   - function foo(a, b = 1) { ... }   (default is last)
//   - function foo(a = 1, b = 2) { ... } (all defaults)
type SLP208 struct{}

func (SLP208) ID() string                { return "SLP208" }
func (SLP208) DefaultSeverity() Severity { return SeverityWarn }
func (SLP208) Description() string {
	return "default parameter before required parameter — move defaults to the end"
}

// Matches the start of a function/arrow signature up to the opening paren.
var slp208FuncStartRe = regexp.MustCompile(
	`(?:function\s+\w+|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?)\(`)

// Matches a single parameter with a default value: `name = value`.
var slp208DefaultParamRe = regexp.MustCompile(`\w+\s*=\s*[^,]+`)

// Matches comment lines.
var slp208CommentRe = regexp.MustCompile(`^\s*(//|/\*|\*)`)

func (r SLP208) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if !isJSOrTSFile(f.Path) {
			continue
		}
		for _, ln := range f.AddedLines() {
			trimmed := strings.TrimSpace(ln.Content)
			if trimmed == "" || slp208CommentRe.MatchString(trimmed) {
				continue
			}
			if finding := r.checkLine(f.Path, ln); finding != nil {
				out = append(out, *finding)
			}
		}
	}
	return out
}

func (r SLP208) checkLine(path string, ln diff.Line) *Finding {
	loc := slp208FuncStartRe.FindStringIndex(ln.Content)
	if len(loc) > 1 {
		start := loc[1] - 1
		// Extract the balanced param list starting from the opening `(`.
		params, ok := extractBalancedParens(ln.Content[start:])
		if ok && params != "" {
			return r.checkParams(path, ln, params)
		}
	}
	return nil
}

func (r SLP208) checkParams(path string, ln diff.Line, params string) *Finding {
	// Split params by comma, respecting nested parens/brackets.
	parts := splitParams(params)
	if len(parts) < 2 {
		return nil
	}

	// Find the first default param index and the last required param index.
	firstDefault := -1
	lastRequired := -1
	for i, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || trimmed == "..." {
			// Rest params are always last, skip.
			continue
		}
		if slp208DefaultParamRe.MatchString(trimmed) {
			if firstDefault == -1 {
				firstDefault = i
			}
		} else {
			lastRequired = i
		}
	}

	// Problem: a default param appears before a required param.
	if firstDefault >= 0 && lastRequired >= 0 && firstDefault < lastRequired {
		return &Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			File:     path,
			Line:     ln.NewLineNo,
			Message:  "default parameter before required parameter — move defaults to the end of the parameter list",
			Snippet:  strings.TrimSpace(ln.Content),
		}
	}
	return nil
}

// extractBalancedParens extracts the content between balanced parentheses
// starting from an opening `(`. Returns the inner content and true if
// balanced, or ("", false) if unbalanced.
func extractBalancedParens(s string) (string, bool) {
	if len(s) > 0 && s[0] == '(' {
		depth := 0
		for i, c := range s {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return s[1:i], true
				}
			}
		}
	}
	return "", false
}

// splitParams splits a parameter string by top-level commas.
func splitParams(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, c := range s {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
