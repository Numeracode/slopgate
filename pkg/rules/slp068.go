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
	if len(candidates) < 2 {
		return candidates
	}
	out := []slp068Candidate{candidates[0]}
	for _, candidate := range candidates[1:] {
		last := &out[len(out)-1]
		if candidate.start <= last.end {
			if candidate.end > last.end {
				last.end = candidate.end
			}
			continue
		}
		out = append(out, candidate)
	}
	return out
}

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
		for _, candidate := range slp068CollapseCandidates(candidates) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     candidate.line,
				Message:  "duplicate code block within the same file — extract to helper function",
				Snippet:  strings.TrimSpace(added[candidate.start].Content),
			})
		}
	}
	return out
}
