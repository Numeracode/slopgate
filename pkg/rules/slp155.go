package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP155 flags ALTER TABLE … ADD COLUMN statements that declare a column
// NOT NULL without a DEFAULT value.  Adding a NOT NULL column to a non-empty
// table without a DEFAULT causes an immediate error in PostgreSQL and most
// other databases, because existing rows have no value to fill in.
//
// Safe patterns that are NOT flagged:
//   - ADD COLUMN … NOT NULL DEFAULT <value>
//   - ADD COLUMN … NOT NULL  (when the column is created on a brand-new table
//     in the same diff — detected by the presence of a CREATE TABLE for the
//     same table name earlier in the file)
//
// Languages: SQL migration files (.sql in a migrations/ directory).
type SLP155 struct{}

func (SLP155) ID() string                { return "SLP155" }
func (SLP155) DefaultSeverity() Severity { return SeverityWarn }
func (SLP155) Description() string {
	return "migration adds NOT NULL column without DEFAULT — existing rows will fail to satisfy the constraint"
}

// matches an ADD COLUMN line (with optional IF NOT EXISTS).
var slp155AddColRe = regexp.MustCompile(
	`(?i)\badd\s+column(?:\s+if\s+not\s+exists)?\s+([a-z0-9_"]+)`)

// detects NOT NULL anywhere on the line (after stripping comments/strings).
var slp155NotNullRe = regexp.MustCompile(`(?i)\bnot\s+null\b`)

// detects DEFAULT anywhere on the line.
var slp155DefaultRe = regexp.MustCompile(`(?i)\bdefault\b`)

// matches CREATE TABLE [IF NOT EXISTS] <name> to track newly-created tables.
var slp155CreateTableRe = regexp.MustCompile(
	`(?i)\bcreate\s+table(?:\s+if\s+not\s+exists)?\s+([a-z0-9_"]+)`)

// matches ALTER TABLE <name>.
var slp155AlterTableRe = regexp.MustCompile(
	`(?i)\balter\s+table\s+([a-z0-9_"]+)`)

func (r SLP155) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp126IsMigrationSQL(f.Path) {
			continue
		}

		// Collect all newly-created table names in this diff file so we can
		// skip ADD COLUMN NOT NULL on a freshly-created table (safe).
		newTables := map[string]bool{}
		currentAlterTable := ""

		for _, ln := range f.AddedLines() {
			content := strings.TrimSpace(stripCommentAndStrings(ln.Content))
			if content == "" {
				continue
			}
			lower := strings.ToLower(content)

			if m := slp155CreateTableRe.FindStringSubmatch(lower); m != nil {
				newTables[strings.Trim(m[1], `"`)] = true
			}

			if m := slp155AlterTableRe.FindStringSubmatch(lower); m != nil {
				currentAlterTable = strings.Trim(m[1], `"`)
			}

			m := slp155AddColRe.FindStringSubmatch(lower)
			if m == nil {
				continue
			}

			if !slp155NotNullRe.MatchString(lower) {
				continue // nullable column — fine
			}
			if slp155DefaultRe.MatchString(lower) {
				continue // has a DEFAULT — fine
			}

			// Skip if the target table is brand-new in this diff.
			if newTables[currentAlterTable] {
				continue
			}

			colName := strings.Trim(m[1], `"`)
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     ln.NewLineNo,
				Message: "column '" + colName + "' is NOT NULL without a DEFAULT — " +
					"existing rows will violate the constraint; add DEFAULT or backfill first",
				Snippet: strings.TrimSpace(ln.Content),
			})
		}
	}
	return out
}
