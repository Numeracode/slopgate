package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP219 flags access to shared state fields (e.g. *Manager, *Store, *State)
// in an HTTP handler without holding a mutex. Whimsy PR #1952 reviewers flagged
// `s.SnapshotManager` read/write in handlers that didn't call s.mu.Lock() or
// s.mu.RLock() anywhere in the hunk.
//
// Heuristic:
//   - Added line accesses a field named <prefix>Manager|Store|State|Config|Registry
//   - Same hunk is in an HTTP handler (has http.ResponseWriter / *http.Request)
//   - Hunk does NOT contain any .Lock() / .RLock() / atomic.Load*/Store* call
type SLP219 struct{}

func (SLP219) ID() string                { return "SLP219" }
func (SLP219) DefaultSeverity() Severity { return SeverityWarn }
func (SLP219) Description() string {
	return "accessing shared-state field in handler without lock — data race risk"
}

// sharedFieldRe matches s.X, r.X, h.X where X looks like shared mutable state.
var sharedFieldRe = regexp.MustCompile(`\b([a-zA-Z])\.(Manager|Store|State|Config|Registry|Cache|Pool|Snapshot[A-Z][a-zA-Z]*)\b`)

// handlerContextRe identifies an HTTP handler via parameter types.
var handlerContextRe = regexp.MustCompile(`http\.ResponseWriter|\*http\.Request`)

// lockCallRe matches mutex acquisition or atomic ops.
var lockCallRe = regexp.MustCompile(`\.(Lock|Unlock|RLock|RUnlock)\(|atomic\.(Load|Store|Add|Swap|CompareAndSwap)`)

func (r SLP219) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			var addedText strings.Builder
			var addedLines []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					addedText.WriteString(ln.Content)
					addedText.WriteByte('\n')
					addedLines = append(addedLines, ln)
				}
			}
			if len(addedLines) == 0 {
				continue
			}

			hunkStr := addedText.String()
			// Only care if hunk is in handler context.
			if !handlerContextRe.MatchString(hunkStr) {
				continue
			}
			// If hunk takes a lock, assume safe.
			if lockCallRe.MatchString(hunkStr) {
				continue
			}

			for _, ln := range addedLines {
				if sharedFieldRe.MatchString(ln.Content) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     ln.NewLineNo,
						Message:  "shared-state field access in handler without .Lock()/atomic — concurrent requests race",
						Snippet:  strings.TrimSpace(ln.Content),
					})
				}
			}
		}
	}
	return out
}
