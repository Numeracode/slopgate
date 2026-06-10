package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP214 flags React Query result access without error checking.
// When code accesses `.data` on a React Query result (useQuery, useMutation)
// without first checking `.isError` or `.error`, any fetch failure will
// result in accessing undefined data, typically causing a runtime crash.
//
// Reviewers frequently flag this pattern as a silent bug — the happy path
// works but the error path crashes silently.
type SLP214 struct{}

func (SLP214) ID() string                { return "SLP214" }
func (SLP214) DefaultSeverity() Severity { return SeverityWarn }
func (SLP214) Description() string {
	return "React Query data access without error check"
}

// slp214QueryHookRe matches React Query hooks.
var slp214QueryHookRe = regexp.MustCompile(`(?:const|let|var)\s+\w+\s*=\s*(?:useQuery|useMutation|useInfiniteQuery|useSuspenseQuery)\s*\(`)

// slp214DataAccessRe matches accessing .data on a query result variable.
var slp214DataAccessRe = regexp.MustCompile(`(\w+)\.data\b`)

// slp214ErrorCheckRe checks for error handling patterns.
var slp214ErrorCheckRe = regexp.MustCompile(`\.isError\b|\.error\b|\.status\s*===?\s*["']error["']|onError\b|if\s*\(.*error`)

// slp214VarNameRe extracts the variable name from a declaration.
var slp214VarNameRe = regexp.MustCompile(`(?:const|let|var)\s+(\w+)`)

func (r SLP214) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isJSOrTSFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			addedLines := collectAddedLines(h)

			// First pass: find query hook variable names
			queryVars := map[string]bool{}
			for _, ln := range addedLines {
				matches := slp214QueryHookRe.FindStringSubmatch(ln.Content)
				if len(matches) > 0 {
					// Extract variable name: const xxx = useQuery(...)
					vm := slp214VarNameRe.FindStringSubmatch(ln.Content)
					if len(vm) > 1 {
						queryVars[vm[1]] = true
					}
				}
			}

			if len(queryVars) == 0 {
				continue
			}

			// Second pass: check data access without error checks
			hasErrorCheck := map[string]bool{}
			for _, ln := range addedLines {
				if slp214ErrorCheckRe.MatchString(ln.Content) {
					// Mark any query vars in this block as having error handling
					for v := range queryVars {
						if strings.Contains(ln.Content, v) {
							hasErrorCheck[v] = true
						}
					}
				}
			}

			for _, ln := range addedLines {
				dataMatches := slp214DataAccessRe.FindStringSubmatch(ln.Content)
				if len(dataMatches) > 1 && queryVars[dataMatches[1]] && !hasErrorCheck[dataMatches[1]] {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  fmt.Sprintf("%s.data accessed without checking for query error — add isError/error guard", dataMatches[1]),
						Snippet:  ln.Content,
					})
				}
			}
		}
	}
	return out
}
