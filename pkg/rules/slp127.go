package rules

import (
	"path/filepath"
	"regexp"
	"sort"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP127 flags slopgate rule implementation changes without matching test-file
// updates in the same diff.
type SLP127 struct{}

func (SLP127) ID() string                { return "SLP127" }
func (SLP127) DefaultSeverity() Severity { return SeverityWarn }
func (SLP127) Description() string {
	return "slopgate rule implementation changed without corresponding test diff update"
}

var slp127RuleImplPathRe = regexp.MustCompile(`^pkg/rules/slp\d+\.go$`)
var slp127RuleTestPathRe = regexp.MustCompile(`^pkg/rules/slp\d+_test\.go$`)

func (r SLP127) Check(d *diff.Diff) []Finding {
	var out []Finding
	changedRuleFirstLine := map[string]int{}
	changedRuleSnippet := map[string]string{}
	changedTests := map[string]bool{}
	files := d.AllFiles()

	for _, f := range files {
		if f.IsDelete {
			continue
		}
		path := filepath.ToSlash(f.Path)
		added := f.AddedLines()

		if slp127RuleTestPathRe.MatchString(path) && len(added) > 0 {
			changedTests[path] = true
			continue
		}
	}

	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		path := filepath.ToSlash(f.Path)
		if !slp127RuleImplPathRe.MatchString(path) || slp127RuleTestPathRe.MatchString(path) {
			continue
		}
		added := f.AddedLines()
		if len(added) == 0 {
			continue
		}
		changedRuleFirstLine[path] = added[0].NewLineNo
		changedRuleSnippet[path] = added[0].Content
	}

	paths := make([]string, 0, len(changedRuleFirstLine))
	for p := range changedRuleFirstLine {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		expectedTest := p[:len(p)-len(".go")] + "_test.go"
		if changedTests[expectedTest] {
			continue
		}
		out = append(out, Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			File:     p,
			Line:     changedRuleFirstLine[p],
			Message:  "rule file changed without matching test updates in this diff — add regression coverage in " + expectedTest,
			Snippet:  changedRuleSnippet[p],
		})
	}

	return out
}
