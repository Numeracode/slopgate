package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP225 flags new goroutines that mutate captured shared state without an
// obvious synchronization guard in the same hunk. This is a heuristic aimed
// at catching:
//
//   go func() {
//       someMap[key] = value       // map write
//       someField = value          // package-level var or captured var
//       obj.Field = value          // struct field write
//   }()
//
// The rule is diff-based: it only examines added lines. A guard is accepted
// when the hunk also contains one of: sync.Mutex / sync.RWMutex,
// atomic.*, channel send/recv on a guard channel, WaitGroup, or the guard is
// present in the immediate enclosing function scope (e.g. `mu.Lock()` /
// `defer mu.Unlock()`).
//
// Severity: warn (heuristic).
type SLP225 struct{}

func (SLP225) ID() string                { return "SLP225" }
func (SLP225) DefaultSeverity() Severity { return SeverityWarn }
func (SLP225) Description() string {
	return "goroutine mutates shared state without visible synchronization"
}

// goroutineStartRe matches the start of an anonymous goroutine.
var goroutineStartRe = regexp.MustCompile(`\bgo\s+func\s*\(\s*\)\s*\{|\bgo\s+func\s*\(\s*\)\s*\(\s*\)`)

// mapWriteRe matches map writes via index assignment.
var mapWriteRe = regexp.MustCompile(`\b(\w+)\s*\[[^\]]+\]\s*=\s*`)

// fieldWriteRe matches struct field writes on captured variables (obj.Field =).
var fieldWriteRe = regexp.MustCompile(`\b(\w+)\.\w+\s*=\s*([^=].*)?$`)

// packageVarWriteRe matches assignments to package-level or outer-scope
// variables that are not local declarations.
var packageVarWriteRe = regexp.MustCompile(`\b(\w+)\s*=\s*([^=].*)?$`)

// syncGuardRe matches mutex, atomic, channel, and WaitGroup usage.
var syncGuardRe = regexp.MustCompile(`\bsync\.(Mutex|RWMutex|WaitGroup)\b|\b(Mutex|RWMutex|WaitGroup)\b|\.Lock\(\s*\)|\.Unlock\(\s*\)|\.RLock\(\s*\)|\.RUnlock\(\s*\)|atomic\.(Add|Store|CompareAndSwap|Load|Swap)|\b(make\s*\(\s*chan|chan\s*\w+\s*[\|])|\b\w+\s*<-`)

func (r SLP225) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			added := hunkAddedLines(h)
			if len(added) == 0 {
				continue
			}

			addedText := ""
			for _, ln := range added {
				addedText += ln.Content + "\n"
			}
			hasSync := syncGuardRe.MatchString(addedText)

			// Locate each goroutine in the added hunk and check for writes
			// inside it.
			for i, ln := range added {
				if !goroutineStartRe.MatchString(ln.Content) {
					continue
				}
				start := i
				end := goroutineEnd(added, i)
				if end == -1 {
					// If we cannot find a closing brace, still scan until the
					// end of the added hunk.
					end = len(added) - 1
				}

				// If the surrounding function already has a guard declared,
				// treat the block as guarded unless the goroutine captures it
				// without using it. We keep it simple: any sync construct in the
				// hunk satisfies the guard requirement.
				if hasSync {
					continue
				}

				for j := start + 1; j < end; j++ {
					content := added[j].Content
					if mapWriteRe.MatchString(content) ||
						fieldWriteRe.MatchString(content) ||
						packageVarWriteRe.MatchString(content) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     added[j].NewLineNo,
							Message:  "goroutine writes shared state without visible synchronization guard",
							Snippet:  strings.TrimSpace(content),
						})
						break // one finding per goroutine is enough
					}
				}
			}
		}
	}
	return out
}

func hunkAddedLines(h diff.Hunk) []diff.Line {
	var added []diff.Line
	for _, ln := range h.Lines {
		if ln.Kind == diff.LineAdd {
			added = append(added, ln)
		}
	}
	return added
}

// goroutineEnd returns the index in `added` that closes the goroutine block
// started at idx. It does naive brace counting across added lines only.
func goroutineEnd(added []diff.Line, idx int) int {
	depth := 0
	inBody := false
	for i := idx; i < len(added); i++ {
		content := added[i].Content
		open := strings.Count(content, "{")
		close := strings.Count(content, "}")
		if i == idx {
			open--
		}
		if open > 0 {
			inBody = true
		}
		depth += open - close
		if inBody && depth <= 0 {
			return i
		}
	}
	return -1
}
