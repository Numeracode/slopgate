package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP209 detects async arrow functions that return a value on some code
// paths but not on all — typically the last statement before `}` is not
// a `return`. This catches a common bug: error-handling branches return
// early but the happy path falls through with `undefined`.
//
// The rule fires when:
//   - An async arrow function body contains at least one `return` (so it
//     is intended to return a value), AND
//   - The last non-empty line before the closing `}` is not a `return`,
//     `throw`, `break`, `continue`, or another closing brace.
//
// This is a high-signal heuristic: if the function never returns anything,
// it is likely a side-effect-only handler (not flagged). But if it returns
// on *some* paths and misses the fall-through, that is almost always a bug.
//
// Flagged patterns:
//
//	const getUser = async (id) => {
//	  const user = await db.find(id)
//	  if (!user) return null
//	  user  // missing return — falls through with undefined
//	}
//
//	const getUser = async (id) => {
//	  try {
//	    return await db.find(id)
//	  } catch (e) {
//	    log(e)
//	  }
//	}
//
// Not flagged:
//
//	const handler = async (req, res) => {
//	  const data = await fetch(req.url)
//	  res.json(data)
//	}
//
//	const getUser = async (id) => {
//	  return await db.find(id)
//	}
type SLP209 struct{}

func (SLP209) ID() string                { return "SLP209" }
func (SLP209) DefaultSeverity() Severity { return SeverityWarn }
func (SLP209) Description() string {
	return "async arrow function returns on some paths but not all — missing return at end of body"
}

// Matches async arrow function opening: `async (...) => {` or `async () => {`.
// Uses a greedy match up to `) =>` to handle TypeScript type annotations.
var slp209AsyncArrowRe = regexp.MustCompile(`\basync\s*\(.*?\)\s*(?::\s*[^{]+)?\s*=>\s*\{`)

// Matches a return statement.
var slp209ReturnRe = regexp.MustCompile(`\breturn\b`)

// Matches terminating statements (return, throw, break, continue).
var slp209TerminatingRe = regexp.MustCompile(`\b(return|throw|break|continue)\b`)

// Matches lines that are only closing braces or whitespace.
var slp209CloseBraceRe = regexp.MustCompile(`^\s*\}`)

// Matches comment lines.
var slp209CommentRe = regexp.MustCompile(`^\s*(//|/\*|\*)`)

func (r SLP209) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if !isJSOrTSFile(f.Path) {
			continue
		}
		out = append(out, r.checkFile(&f)...)
	}
	return out
}

func (r SLP209) checkFile(f *diff.File) []Finding {
	if f == nil {
		return nil
	}
	var out []Finding
	for _, h := range f.Hunks {
		out = append(out, r.checkHunk(f.Path, h)...)
	}
	return out
}

func (r SLP209) checkHunk(path string, h diff.Hunk) []Finding {
	var out []Finding
	lines := h.Lines
	for i, ln := range lines {
		if ln.Kind != diff.LineAdd {
			continue
		}
		trimmed := strings.TrimSpace(ln.Content)
		if !slp209AsyncArrowRe.MatchString(trimmed) {
			continue
		}

		// Found an async arrow function opening. Now collect the body
		// until the matching closing brace.
		body, endLine := r.collectBody(lines, i)
		if len(body) == 0 {
			continue
		}

		// Check: does the body contain at least one return?
		hasReturn := false
		for _, bl := range body {
			bt := strings.TrimSpace(bl)
			if slp209ReturnRe.MatchString(bt) {
				hasReturn = true
				break
			}
		}
		if !hasReturn {
			continue
		}

		// Check: is the last non-empty, non-brace line a terminating statement?
		lastStmt := r.lastStatement(body)
		if lastStmt == "" {
			continue
		}
		if slp209TerminatingRe.MatchString(lastStmt) {
			continue
		}

		out = append(out, Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			File:     path,
			Line:     ln.NewLineNo,
			Message:  "async arrow function returns on some paths but not all — add a return statement at the end",
			Snippet:  trimmed,
		})
		_ = endLine
	}
	return out
}

// collectBody gathers lines from the async arrow opening `{` to the matching `}`.
func (r SLP209) collectBody(lines []diff.Line, startIdx int) ([]string, int) {
	depth := 0
	var body []string
	entered := false
	for i := startIdx; i < len(lines); i++ {
		// Skip deleted lines — they don't appear in the new file.
		if lines[i].Kind == diff.LineDelete {
			continue
		}
		content := lines[i].Content
		firstOpen := -1
		for j, c := range content {
			switch c {
			case '{':
				depth++
				if !entered {
					entered = true
					firstOpen = j
				}
			case '}':
				depth--
			}
		}
		if entered {
			// For the opening line, extract only the content after `{`.
			// For subsequent lines, include the full content.
			if i == startIdx && firstOpen >= 0 && firstOpen+1 < len(content) {
				body = append(body, content[firstOpen+1:])
			} else if i > startIdx {
				body = append(body, content)
			}
		}
		if entered && depth == 0 {
			return body, i
		}
	}
	// If we hit the end without closing, still return what we have.
	if len(lines) > 0 {
		return body, len(lines) - 1
	}
	return body, 0
}

// lastStatement returns the last non-empty, non-brace-only line in the body.
func (r SLP209) lastStatement(body []string) string {
	for i := len(body) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(body[i])
		if trimmed == "" || slp209CommentRe.MatchString(trimmed) {
			continue
		}
		if slp209CloseBraceRe.MatchString(trimmed) {
			continue
		}
		// Strip trailing `}` from inline body (single-line functions).
		trimmed = strings.TrimRight(trimmed, " }")
		if trimmed == "" {
			continue
		}
		return trimmed
	}
	return ""
}
