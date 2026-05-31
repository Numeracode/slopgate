package rules

import (
	"fmt"
	"regexp"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP212 flags potential double-submit race conditions where a boolean
// state guard is set before an async operation. In React, state updates
// are batched and re-renders happen asynchronously, so setting
// setIsSubmitting(true) before an await does not guarantee the guard
// is in place when the user clicks again during the same render cycle.
//
// This is a known class of bug in React: the state guard only takes
// effect after the next render, leaving a window for double-submit.
type SLP212 struct{}

func (SLP212) ID() string                { return "SLP212" }
func (SLP212) DefaultSeverity() Severity { return SeverityWarn }
func (SLP212) Description() string {
	return "boolean state guard before async may allow double-submit"
}

// slp212GuardSetRe matches patterns like setIsX(true) or setSubmitting(true).
var slp212GuardSetRe = regexp.MustCompile(`set(Is)?\w+\(\s*true\s*\)`)

// slp212GuardCheckRe matches patterns like if (!isX) or if (!submitting).
var slp212GuardCheckRe = regexp.MustCompile(`if\s*\(\s*!?(\w+)\s*\)`)

// slp212AwaitRe matches await in the same block.
var slp212AwaitRe = regexp.MustCompile(`\bawait\b`)

func (r SLP212) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isSourceLikeFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			addedLines := collectAddedLines(h)
			for i, ln := range addedLines {
				if !slp212GuardCheckRe.MatchString(ln.Content) {
					continue
				}
				// Look ahead for: setState(true) soon followed by await
				hasGuardSet := false
				hasAwait := false
				for j := i + 1; j < len(addedLines) && j <= i+5; j++ {
					if slp212GuardSetRe.MatchString(addedLines[j].Content) {
						hasGuardSet = true
					}
					if slp212AwaitRe.MatchString(addedLines[j].Content) {
						hasAwait = true
					}
				}
				if hasGuardSet && hasAwait {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  fmt.Sprintf("boolean state guard before async — consider using a ref or disabling the button to prevent double-submit during the render gap"),
						Snippet:  ln.Content,
					})
				}
			}
		}
	}
	return out
}