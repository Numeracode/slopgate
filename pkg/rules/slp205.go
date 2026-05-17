package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP205 flags OpenAPI path merge order that lets generated or hardcoded
// path maps override richer JSDoc-derived spec.paths annotations.
type SLP205 struct{}

// ID returns the stable rule identifier.
func (SLP205) ID() string { return "SLP205" }

// DefaultSeverity returns the default finding severity.
func (SLP205) DefaultSeverity() Severity { return SeverityWarn }

// Description returns a short rule summary for rule catalogs.
func (SLP205) Description() string {
	return "OpenAPI path merge order lets generated paths override annotated spec paths"
}

var (
	slp205SpecPathsAssign       = regexp.MustCompile(`\bspec\.paths\s*=\s*\{`)
	slp205SpecPathsSpread       = regexp.MustCompile(`\.\.\.\s*\(?\s*spec\.paths\s*(?:\|\|\s*\{\s*\})?\s*\)?`)
	slp205GeneratedPathsSpread  = regexp.MustCompile(`(?i)\.\.\.\s*(?:oas\d+paths|openapi\w*paths|generated\w*paths|hardcoded\w*paths)\b`)
	slp205CommentOnlyLinePrefix = regexp.MustCompile(`^\s*(?://|/\*|\*|#)`)
)

type slp205Line struct {
	content   string
	clean     string
	kind      diff.LineKind
	newLineNo int
	order     int
	depthBase int
}

type slp205Event struct {
	kind  string
	line  slp205Line
	pos   int
	depth int
}

// Check scans JS/TS OpenAPI assembly diffs for unsafe path map spread order.
func (r SLP205) Check(d *diff.Diff) []Finding {
	var out []Finding
	if d == nil {
		return out
	}

	for _, f := range d.Files {
		// Limit this rule to source files where OpenAPI specs are assembled.
		if f.IsDelete || !isJSOrTSFile(f.Path) || isTestFile(f.Path) ||
			isGeneratedArtifactPath(f.Path) || isOpenAPIArtifactPath(f.Path) {
			continue
		}

		for _, h := range f.Hunks {
			for i := range h.Lines {
				ln := h.Lines[i]
				if ln.Kind == diff.LineDelete ||
					slp205CommentOnlyLinePrefix.MatchString(ln.Content) ||
					!slp205SpecPathsAssign.MatchString(ln.Content) {
					continue
				}

				block := slp205CollectObjectBlock(h.Lines, i)
				if len(block) == 0 || !slp205HasAddedRelevantLine(block) {
					continue
				}
				if generatedLine, ok := slp205BadPathMergeOrder(block); ok {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     generatedLine.newLineNo,
						Message:  "OpenAPI generated path map is spread after spec.paths, overriding richer JSDoc annotations; spread spec.paths last",
						Snippet:  strings.TrimSpace(generatedLine.content),
					})
					break
				}
			}
		}
	}
	return out
}

func slp205CollectObjectBlock(lines []diff.Line, start int) []slp205Line {
	if len(lines) == 0 || start < 0 || start >= len(lines) {
		return nil
	}
	var out []slp205Line
	depth := 0
	started := false
	for i := start; i < len(lines); i++ {
		ln := lines[i]
		if ln.Kind == diff.LineDelete {
			continue
		}
		content := ln.Content
		cleanContent := slp205StripJSCommentsOutsideStrings(content)
		lineDepthBase := depth
		var lineDelta int
		if !started {
			open := strings.Index(cleanContent, "{")
			if open < 0 {
				continue
			}
			started = true
			lineDepthBase = 0
			lineDelta = slp205BraceDelta(cleanContent[open:])
		} else {
			lineDelta = slp205BraceDelta(cleanContent)
		}

		out = append(out, slp205Line{
			content:   content,
			clean:     cleanContent,
			kind:      ln.Kind,
			newLineNo: ln.NewLineNo,
			order:     i,
			depthBase: lineDepthBase,
		})
		depth += lineDelta
		if started && depth <= 0 {
			break
		}
	}
	return out
}

func slp205BraceDelta(content string) int {
	if content == "" {
		return 0
	}
	delta := 0
	for _, r := range content {
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

func slp205HasAddedRelevantLine(lines []slp205Line) bool {
	if len(lines) == 0 {
		return false
	}
	for _, ln := range lines {
		if ln.kind != diff.LineAdd || slp205CommentOnlyLinePrefix.MatchString(ln.content) {
			continue
		}
		if slp205SpecPathsAssign.MatchString(ln.clean) ||
			slp205SpecPathsSpread.MatchString(ln.clean) ||
			slp205GeneratedPathsSpread.MatchString(ln.clean) {
			return true
		}
	}
	return false
}

func slp205BadPathMergeOrder(lines []slp205Line) (slp205Line, bool) {
	if len(lines) == 0 {
		return slp205Line{}, false
	}
	events := slp205MergeEvents(lines)
	specSeen := false
	for _, event := range events {
		switch event.kind {
		case "spec":
			if !specSeen {
				specSeen = true
			}
		case "generated":
			if specSeen {
				return event.line, true
			}
		}
	}
	return slp205Line{}, false
}

func slp205MergeEvents(lines []slp205Line) []slp205Event {
	if len(lines) == 0 {
		return nil
	}
	var candidates []slp205Event
	baseDepth := -1
	for _, ln := range lines {
		if slp205CommentOnlyLinePrefix.MatchString(ln.content) {
			continue
		}
		specMatches := slp205SpecPathsSpread.FindAllStringIndex(ln.clean, -1)
		for i := range specMatches {
			match := specMatches[i]
			if len(match) < 2 {
				continue
			}
			pos, ok := slp205MatchStart(match)
			if !ok {
				continue
			}
			depth := ln.depthBase + slp205BraceDepthAt(ln.clean, pos)
			candidates = append(candidates, slp205Event{kind: "spec", line: ln, pos: pos, depth: depth})
			if baseDepth < 0 || depth < baseDepth {
				baseDepth = depth
			}
		}
		generatedMatches := slp205GeneratedPathsSpread.FindAllStringIndex(ln.clean, -1)
		for i := range generatedMatches {
			match := generatedMatches[i]
			if len(match) < 2 {
				continue
			}
			pos, ok := slp205MatchStart(match)
			if !ok {
				continue
			}
			depth := ln.depthBase + slp205BraceDepthAt(ln.clean, pos)
			candidates = append(candidates, slp205Event{kind: "generated", line: ln, pos: pos, depth: depth})
			if baseDepth < 0 || depth < baseDepth {
				baseDepth = depth
			}
		}
	}
	events := make([]slp205Event, 0, len(candidates))
	for _, event := range candidates {
		if event.depth == baseDepth {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].line.order == events[j].line.order {
			return events[i].pos < events[j].pos
		}
		return events[i].line.order < events[j].line.order
	})
	return events
}

func slp205MatchStart(match []int) (int, bool) {
	// regexp.FindAllStringIndex returns [start, end]; keep access centralized
	// so scanner rules see the defensive checks before any position is used.
	if len(match) < 2 {
		return 0, false
	}
	for _, value := range match {
		return value, true
	}
	return 0, false
}

func slp205BraceDepthAt(content string, offset int) int {
	// Count braces before the match while ignoring quoted strings; callers add
	// the cross-line object depth captured when the block was collected.
	if content == "" || offset <= 0 {
		return 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	depth := 0
	var quote rune
	escaped := false
	for byteOffset, r := range content {
		if byteOffset >= offset {
			break
		}
		if quote != 0 {
			// Inside a string literal, only escapes and the closing quote matter.
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			// Enter a quoted region so braces in path strings are ignored.
			quote = r
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func slp205StripJSCommentsOutsideStrings(content string) string {
	if content == "" {
		return ""
	}
	out := []byte(content)
	state := slp205StringScanState{}
	for i := 0; i < len(out); i++ {
		ch := out[i]
		// Preserve byte positions by blanking comments after skipping quoted source text.
		if state.consume(ch) {
			continue
		}
		if slp205StartsComment(out, i, '/') {
			slp205BlankRange(out, i, len(out))
			return string(out)
		}
		if slp205StartsComment(out, i, '*') {
			i = slp205BlankBlockComment(out, i)
		}
	}
	return string(out)
}

type slp205StringScanState struct {
	quote   byte
	escaped bool
}

func (s *slp205StringScanState) consume(ch byte) bool {
	// Inside a string, comments and braces are source text, not structure.
	if s.quote != 0 {
		if s.escaped {
			s.escaped = false
			return true
		}
		if ch == '\\' {
			s.escaped = true
			return true
		}
		if ch == s.quote {
			s.quote = 0
		}
		return true
	}
	if ch == '\'' || ch == '"' || ch == '`' {
		s.quote = ch
		return true
	}
	return false
}

func slp205StartsComment(content []byte, offset int, next byte) bool {
	if len(content) == 0 || offset < 0 || offset+1 >= len(content) {
		return false
	}
	return content[offset] == '/' && content[offset+1] == next
}

func slp205BlankBlockComment(content []byte, offset int) int {
	if len(content) == 0 || !slp205StartsComment(content, offset, '*') {
		return offset
	}
	slp205BlankRange(content, offset, offset+2)
	i := offset + 2
	for i < len(content) {
		if i+1 < len(content) && content[i] == '*' && content[i+1] == '/' {
			slp205BlankRange(content, i, i+2)
			return i + 1
		}
		content[i] = ' '
		i++
	}
	return len(content) - 1
}

func slp205BlankRange(content []byte, start, end int) {
	// Clamp the caller's range so comment blanking preserves the original line length.
	if len(content) == 0 {
		return
	}
	if start < 0 {
		start = 0
	}
	if end > len(content) {
		end = len(content)
	}
	for i := start; i < end; i++ {
		content[i] = ' '
	}
}
