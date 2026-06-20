//go:build ignore_retired

package rules

import (
	"strconv"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP055 flags Go functions with more than 3 conditionals (if/for/select/switch)
// and zero comment lines added inside the function body.
type SLP055 struct{}

func (SLP055) ID() string                { return "SLP055" }
func (SLP055) DefaultSeverity() Severity { return SeverityInfo }
func (SLP055) Description() string {
	return "complex logic without comments — explain the complex logic"
}

// isConditionalKeyword reports whether a line starts with a Go control-flow keyword.
func isConditionalKeyword(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "if ") || strings.HasPrefix(s, "if(") ||
		strings.HasPrefix(s, "for ") || strings.HasPrefix(s, "for(") ||
		strings.HasPrefix(s, "select {") || s == "select {" ||
		strings.HasPrefix(s, "switch ") || strings.HasPrefix(s, "switch(") || s == "switch {" ||
		strings.HasPrefix(s, "case ") || strings.HasPrefix(s, "default:")
}

func (r SLP055) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isGoFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			out = append(out, scanHunkForComplexLogic(f.Path, h, r.DefaultSeverity(), r.ID())...)
		}
	}
	return out
}

// scanHunkForComplexLogic scans a hunk for added Go functions that have
// >3 conditional keywords but zero added comment lines inside the body.
func scanHunkForComplexLogic(path string, h diff.Hunk, sev Severity, ruleID string) []Finding {
	var out []Finding
	lines := h.Lines
	i := 0
	for i < len(lines) {
		ln := lines[i]
		if ln.Kind != diff.LineAdd {
			i++
			continue
		}
		content := strings.TrimSpace(ln.Content)
		if !strings.HasPrefix(content, "func ") {
			i++
			continue
		}
		depth := strings.Count(ln.Content, "{") - strings.Count(ln.Content, "}")
		if depth <= 0 {
			if i+1 < len(lines) && lines[i+1].Kind == diff.LineAdd {
				depth = strings.Count(lines[i+1].Content, "{") - strings.Count(lines[i+1].Content, "}")
				i++
			}
		}
		if depth <= 0 {
			i++
			continue
		}
		funcName := extractFuncName(content)
		startLine := ln.NewLineNo
		condCount := 0
		commentCount := 0
		if isConditionalKeyword(ln.Content) {
			condCount++
		}
		if strings.HasPrefix(strings.TrimSpace(ln.Content), "//") {
			commentCount++
		}
		j := i + 1
		bodyAllAdded := true
		for j < len(lines) && depth > 0 {
			bl := lines[j]
			if bl.Kind != diff.LineAdd {
				bodyAllAdded = false
				break
			}
			if isConditionalKeyword(bl.Content) {
				condCount++
			}
			if strings.HasPrefix(strings.TrimSpace(bl.Content), "//") {
				commentCount++
			}
			depth += strings.Count(bl.Content, "{") - strings.Count(bl.Content, "}")
			j++
		}
		if bodyAllAdded && depth == 0 && condCount > 3 && commentCount == 0 {
			out = append(out, Finding{
				RuleID:   ruleID,
				Severity: sev,
				File:     path,
				Line:     startLine,
				Message:  "function " + funcName + " has " + strconv.Itoa(condCount) + " conditionals with no comments — explain the complex logic",
				Snippet:  strings.TrimSpace(ln.Content),
			})
		}
		if j > i {
			i = j
		} else {
			i++
		}
	}
	return out
}
