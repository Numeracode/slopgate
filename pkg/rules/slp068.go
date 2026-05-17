package rules

import (
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP068 flags duplicate 8-line code blocks within the same file.
type SLP068 struct{}

func (SLP068) ID() string                { return "SLP068" }
func (SLP068) DefaultSeverity() Severity { return SeverityWarn }
func (SLP068) Description() string {
	return "duplicate logic block within the same file"
}

const slp068Window = 8

type slp068Candidate struct {
	start int
	end   int
	line  int
}

func windowKey(lines []diff.Line, start int) string {
	var b strings.Builder
	for i := start; i < start+slp068Window && i < len(lines); i++ {
		if i > start {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i].Content)
	}
	return b.String()
}

func slp068DuplicateRunLength(lines []diff.Line, left, right int) int {
	if len(lines) == 0 || left < 0 || right < 0 || left >= len(lines) || right >= len(lines) {
		return 0
	}
	run := 0
	for left+run < len(lines) && right+run < len(lines) {
		if lines[left+run].Content != lines[right+run].Content {
			break
		}
		run++
	}
	return run
}

func slp068CollapseCandidates(candidates []slp068Candidate) []slp068Candidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates
	}
	// Merge overlapping duplicate windows so one repeated block yields one finding.
	collapsed := append(make([]slp068Candidate, 0, len(candidates)), candidates[:1]...)
	for _, next := range candidates[1:] {
		last := &collapsed[len(collapsed)-1]
		if next.start <= last.end {
			if next.end > last.end {
				last.end = next.end
			}
			continue
		}
		collapsed = append(collapsed, next)
	}
	return collapsed
}

// Check reports duplicate code blocks while collapsing overlapping windows into a single finding.
func (r SLP068) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) || !isSourceLikeFile(f.Path) {
			continue
		}
		if isTestFile(f.Path) || isGeneratedArtifactPath(f.Path) || isOpenAPIArtifactPath(f.Path) {
			continue
		}
		added := f.AddedLines()
		if len(added) < slp068Window {
			continue
		}
		seen := make(map[string][]int)
		var candidates []slp068Candidate
		for i := 0; i <= len(added)-slp068Window; i++ {
			key := windowKey(added, i)
			if len(strings.TrimSpace(key)) < 20 {
				continue
			}
			for _, start := range seen[key] {
				runLength := slp068DuplicateRunLength(added, start, i)
				if runLength >= slp068Window {
					candidates = append(candidates, slp068Candidate{
						start: i,
						end:   i + runLength,
						line:  added[i].NewLineNo,
					})
					break
				}
			}
			seen[key] = append(seen[key], i)
		}
		matches := slp068CollapseCandidates(candidates)
		for _, match := range matches {
			var finding Finding
			finding.RuleID = r.ID()
			finding.Severity = r.DefaultSeverity()
			finding.File = f.Path
			finding.Line = match.line
			finding.Message = "duplicate code block within the same file — extract to helper function"
			finding.Snippet = strings.TrimSpace(added[match.start].Content)
			out = append(out, finding)
		}
	}
	return out
}
