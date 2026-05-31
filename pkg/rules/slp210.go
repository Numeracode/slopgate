package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP210 flags conflicting Tailwind CSS utilities — two or more utilities
// that target the same CSS property in the same className string (e.g.
// `text-[22px] text-[18px]` or `text-red-500 text-blue-500`).
//
// Reviewers frequently flag this because the last utility wins silently,
// which is almost always a mistake rather than intentional.
type SLP210 struct{}

func (SLP210) ID() string                { return "SLP210" }
func (SLP210) DefaultSeverity() Severity { return SeverityWarn }
func (SLP210) Description() string {
	return "conflicting Tailwind utilities target the same CSS property"
}

// tailwindPropertyPrefix maps Tailwind utility prefixes to the CSS property
// category they control. Two utilities from the same category in one className
// string indicate a conflict.
//
// Only properties where two values genuinely conflict are listed.
// Complementary utilities (flex + flex-col, text-xs + font-bold) are NOT
// listed because they target different sub-properties.
var tailwindPropertyPrefix = map[string]string{
	"text":    "font-color",
	"bg":      "background-color",
	"rounded": "border-radius",
	"shadow":  "box-shadow",
	"opacity": "opacity",
	"z":       "z-index",
	"ring":    "ring",
	"outline": "outline",
	"scale":   "transform-scale",
	"rotate":  "transform-rotate",
	"translate": "transform-translate",
	"skew":    "transform-skew",
	"overflow": "overflow",
	"cursor":  "cursor",
}

// slp210ClassNameRe matches className={...} or className="..." patterns.
var slp210ClassNameRe = regexp.MustCompile(`className\s*=\s*[{"]\s*([^}"]+)\s*[}"]`)

func (r SLP210) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isJSOrTSFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				matches := slp210ClassNameRe.FindAllStringSubmatch(ln.Content, -1)
				for _, m := range matches {
					if len(m) < 2 {
						continue
					}
					classStr := m[1]
					conflicts := findTailwindConflicts(classStr)
					for _, conflict := range conflicts {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  fmt.Sprintf("conflicting Tailwind utilities: %s (both target %s)", conflict.utilities, conflict.property),
							Snippet:  ln.Content,
						})
					}
				}
			}
		}
	}
	return out
}

type tailwindConflict struct {
	utilities string
	property  string
}

func findTailwindConflicts(classStr string) []tailwindConflict {
	var conflicts []tailwindConflict
	seen := map[string][]string{} // property -> list of utilities

	tokens := strings.Fields(classStr)
	for _, tok := range tokens {
		// Strip arbitrary value prefix for matching: text-[22px] -> text
		base := tok
		if idx := strings.Index(tok, "-"); idx > 0 {
			base = tok[:idx]
		}
		if prop, ok := tailwindPropertyPrefix[base]; ok {
			seen[prop] = append(seen[prop], tok)
		}
	}

	for prop, utilities := range seen {
		if len(utilities) > 1 {
			conflicts = append(conflicts, tailwindConflict{
				utilities: strings.Join(utilities, " + "),
				property:  prop,
			})
		}
	}
	return conflicts
}