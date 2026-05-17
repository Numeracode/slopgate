package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP151 flags an orphaned test: a test file that still calls a
// function, method, or class which the same diff removed from a
// non-test source file and did not re-add or rename in place.
//
// Rationale: when an AI agent renames or deletes a symbol it often
// leaves behind the test that exercised it, producing a test that no
// longer compiles or references dead code.
type SLP151 struct{}

func (SLP151) ID() string                { return "SLP151" }
func (SLP151) DefaultSeverity() Severity { return SeverityWarn }
func (SLP151) Description() string {
	return "test references a symbol removed from source in the same diff — orphaned test"
}

var (
	slp151GoDef       = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*[(\[]`)
	slp151JSDef       = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`)
	slp151JSAssignDef = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:function\b|\([^)]*\)\s*=>|[A-Za-z_$][\w$]*\s*=>)`)
	slp151ClassDef    = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)`)
	slp151PyDef       = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)`)
	slp151PyClass     = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
	slp151Call        = regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*\(`)
)

// slp151MinNameLen is the shortest symbol name SLP151 will act on;
// shorter names collide too easily across unrelated packages.
const slp151MinNameLen = 3

// slp151DefNames returns the names defined by a single source line,
// using the definition syntax of the file's language.
func slp151DefNames(line, filePath string) []string {
	var regexes []*regexp.Regexp
	switch {
	case isGoFile(filePath):
		regexes = []*regexp.Regexp{slp151GoDef}
	case isJSOrTSFile(filePath):
		regexes = []*regexp.Regexp{slp151JSDef, slp151JSAssignDef, slp151ClassDef}
	case isPythonFile(filePath):
		regexes = []*regexp.Regexp{slp151PyDef, slp151PyClass}
	}
	var names []string
	for _, re := range regexes {
		if m := re.FindStringSubmatch(line); len(m) > 1 && m[1] != "" {
			names = append(names, m[1])
		}
	}
	return names
}

func slp151IsTestPath(path string) bool {
	return isTestFile(path) || isPythonTestFile(path)
}

func (r SLP151) Check(d *diff.Diff) []Finding {
	removedFrom := map[string]string{} // symbol -> source file it left
	readded := map[string]bool{}       // symbol re-added as a definition

	for _, f := range d.Files {
		if isDocFile(f.Path) || slp151IsTestPath(f.Path) {
			continue
		}
		if !isGoFile(f.Path) && !isJSOrTSFile(f.Path) && !isPythonFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				for _, name := range slp151DefNames(ln.Content, f.Path) {
					switch ln.Kind {
					case diff.LineDelete:
						if _, ok := removedFrom[name]; !ok {
							removedFrom[name] = f.Path
						}
					case diff.LineAdd:
						readded[name] = true
					}
				}
			}
		}
	}

	// A symbol is orphaning only if it was removed and not re-added
	// anywhere in the diff (an in-place edit or a file move keeps the
	// definition line, so its name lands in readded too).
	orphaned := map[string]string{}
	for name, src := range removedFrom {
		if !readded[name] && len(name) >= slp151MinNameLen {
			orphaned[name] = src
		}
	}
	if len(orphaned) == 0 {
		return nil
	}

	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp151IsTestPath(f.Path) {
			continue
		}

		// Calls to a symbol the test file itself defines are local and
		// must not be attributed to the removed source symbol.
		localDefs := map[string]bool{}
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineDelete {
					continue
				}
				for _, name := range slp151DefNames(ln.Content, f.Path) {
					localDefs[name] = true
				}
			}
		}

		flaggedLine := map[int]bool{}
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineDelete {
					continue
				}
				code := stripCommentAndStrings(ln.Content)
				for _, m := range slp151Call.FindAllStringSubmatch(code, -1) {
					if len(m) > 1 {
						name := m[1]
						src, isOrphan := orphaned[name]
						if !isOrphan || localDefs[name] || flaggedLine[ln.NewLineNo] {
							continue
						}
						flaggedLine[ln.NewLineNo] = true
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  fmt.Sprintf("test calls %s(), which this diff removed from %s — orphaned test", name, src),
							Snippet:  strings.TrimSpace(ln.Content),
						})
					}
				}
			}
		}
	}
	return out
}
