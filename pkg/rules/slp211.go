package rules

import (
	"fmt"
	"regexp"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP211 flags setState calls that clear data immediately before an async
// operation, which is a common bug pattern. When state is cleared before
// an async call, the data needed for the operation or for displaying a
// loading state is already gone by the time the promise settles.
//
// Example: setFiles([]) followed by await deleteFiles(files) — files is
// already empty by the time the delete runs.
type SLP211 struct{}

func (SLP211) ID() string                { return "SLP211" }
func (SLP211) DefaultSeverity() Severity { return SeverityWarn }
func (SLP211) Description() string {
	return "setState clears data before async operation that needs it"
}

// slp211SetClearRe matches setState calls that set an empty array or empty object.
var slp211SetClearRe = regexp.MustCompile(`set\w+\(\s*\[\s*\]\s*\)|set\w+\(\s*\{\s*\}\s*\)|set\w+\(\s*null\s*\)|set\w+\(\s*''\s*\)|set\w+\(\s*""\s*\)`)

// slp211AwaitRe matches await or promise chain patterns.
var slp211AwaitRe = regexp.MustCompile(`\bawait\b`)

// Check analyzes consecutive added lines in the same hunk for clear-before-async patterns.
func (r SLP211) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isSourceLikeFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			addedLines := collectAddedLines(h)
			for i, ln := range addedLines {
				if !slp211SetClearRe.MatchString(ln.Content) {
					continue
				}
				// Look at next few lines for an await
				for j := i + 1; j < len(addedLines) && j <= i+3; j++ {
					if slp211AwaitRe.MatchString(addedLines[j].Content) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  fmt.Sprintf("setState clears data on line %d before async operation on line %d — data may be needed by the async call", ln.NewLineNo, addedLines[j].NewLineNo),
							Snippet:  ln.Content,
						})
						break
					}
				}
			}
		}
	}
	return out
}

type lineInfo struct {
	NewLineNo int
	Content   string
}

func collectAddedLines(h diff.Hunk) []lineInfo {
	var lines []lineInfo
	for _, ln := range h.Lines {
		if ln.Kind == diff.LineAdd {
			lines = append(lines, lineInfo{NewLineNo: ln.NewLineNo, Content: ln.Content})
		}
	}
	return lines
}
