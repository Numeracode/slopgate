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

// Matches function/arrow signatures with parameters.
// We look for `=` inside params followed by a param without `=`.
var slp208FuncSigRe = regexp.MustCompile(
	`(?:function\s+\w+|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?)\(([^)]*)\)`)

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
	matches := slp208FuncSigRe.FindStringSubmatch(ln.Content)
	if len(matches) < 2 {
		return nil
	}
	params := matches[1]
	if params == "" {
		return nil
	}

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
