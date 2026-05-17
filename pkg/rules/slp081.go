package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP081 flags TSX/JSX files that use the React namespace without importing it.
// Plain JSX is valid under the automatic runtime, but direct React.* references
// still need an in-scope React binding.
type SLP081 struct{}

func (SLP081) ID() string                { return "SLP081" }
func (SLP081) DefaultSeverity() Severity { return SeverityWarn }
func (SLP081) Description() string {
	return "React namespace used without React import"
}

var (
	slp081ReactNamespacePattern       = regexp.MustCompile(`\bReact\.`)
	slp081ReactDefaultImportPattern   = regexp.MustCompile(`(?ms)^\s*import\s+React(?:\s*,\s*[^;]+)?\s+from\s+["']react["']\s*;?`)
	slp081ReactNamespaceImportPattern = regexp.MustCompile(`(?m)^\s*import\s+\*\s+as\s+React\s+from\s+["']react["']\s*;?`)
)

func (r SLP081) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}

		// Only check TSX files
		if !strings.HasSuffix(strings.ToLower(f.Path), ".tsx") &&
			!strings.HasSuffix(strings.ToLower(f.Path), ".jsx") {
			continue
		}

		// Check if React is imported in visible file content.
		hasReactImport := false
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd && ln.Kind != diff.LineContext {
					continue
				}

				if slp081HasReactImport(ln.Content) {
					hasReactImport = true
					break
				}
			}
			if hasReactImport {
				break
			}
		}
		if !hasReactImport {
			if content, ok := slp007FileContent(d, f.Path); ok && slp081HasReactImport(content) {
				hasReactImport = true
			}
		}

		// Automatic JSX runtime does not require importing React for plain JSX.
		// Only flag explicit React.* namespace usage with no visible React import.
		if !hasReactImport {
			for _, h := range f.Hunks {
				for _, ln := range h.Lines {
					if ln.Kind != diff.LineAdd {
						continue
					}
					if slp081ReactNamespacePattern.MatchString(ln.Content) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "React namespace used without import - add import React from 'react' or import * as React from 'react'",
							Snippet:  strings.TrimSpace(ln.Content),
						})
						goto nextFile
					}
				}
			}
		}
	nextFile:
	}
	return out
}

func slp081HasReactImport(content string) bool {
	return slp081ReactDefaultImportPattern.MatchString(content) ||
		slp081ReactNamespaceImportPattern.MatchString(content)
}
