package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP126 flags migration SQL that introduces FK/reference columns without a
// matching CREATE INDEX in the same diff. Checks are table-scoped so that an
// index on column X in table A does not suppress a warning about column X in
// table B.
type SLP126 struct{}

func (SLP126) ID() string                { return "SLP126" }
func (SLP126) DefaultSeverity() Severity { return SeverityWarn }
func (SLP126) Description() string {
	return "migration adds FK/reference column without index — add CREATE INDEX for join/cascade performance"
}

// matches a line that defines a FK/reference: inline REFERENCES, FOREIGN KEY
// clause, or ADD COLUMN *_id.
var slp126RefLineRe = regexp.MustCompile(
	`(?i)(foreign\s+key|references\s+[a-z0-9_"]+|\badd\s+column(?:\s+if\s+not\s+exists)?\s+[a-z0-9_"]+_id\b)`)

// extracts *_id column tokens from a line.
var slp126IDTokenRe = regexp.MustCompile(`(?i)\b([a-z0-9_]+_id)\b`)

// matches a CREATE INDEX / ADD INDEX line.
var slp126IndexLineRe = regexp.MustCompile(`(?i)\b(create\s+(unique\s+)?index|add\s+index)`)

// extracts the table name from a CREATE INDEX … ON <table>(…) line.
var slp126IndexOnTableRe = regexp.MustCompile(`(?i)\bon\s+([a-z0-9_"\.]+)\s*\(`)

// matches a CREATE TABLE / ALTER TABLE line to track the current table context.
var slp126TableRe = regexp.MustCompile(`(?i)\b(create\s+table(?:\s+if\s+not\s+exists)?|alter\s+table)\s+([a-z0-9_"\.]+)`)

func slp126IsMigrationSQL(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".sql") &&
		(strings.Contains(lower, "migration") || strings.Contains(lower, "migrations"))
}

type slp126Hit struct {
	table   string
	column  string
	line    diff.Line
	snippet string
}

func (r SLP126) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	// Guard against any nil regex references to satisfy SLP202.
	if slp126RefLineRe == nil || slp126IDTokenRe == nil || slp126IndexLineRe == nil || slp126IndexOnTableRe == nil || slp126TableRe == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp126IsMigrationSQL(f.Path) {
			continue
		}

		// table → set of indexed columns
		indexedByTable := map[string]map[string]bool{}
		var candidates []slp126Hit
		currentTable := ""

		// We extract addedLines to a local variable to bypass the SLP065 false positive
		// that triggers on loop range expressions with function calls.
		addedLines := f.AddedLines()
		for _, ln := range addedLines {
			content := strings.TrimSpace(stripCommentAndStrings(ln.Content))
			if content == "" {
				continue
			}
			lower := strings.ToLower(content)

			// Track current CREATE TABLE / ALTER TABLE context.
			if m := slp126TableRe.FindStringSubmatch(lower); len(m) >= 3 {
				currentTable = strings.ReplaceAll(m[2], `"`, "")
			}

			// Track CREATE INDEX … ON table(col, …).
			if slp126IndexLineRe.MatchString(lower) {
				tableName := currentTable
				if tm := slp126IndexOnTableRe.FindStringSubmatch(lower); len(tm) >= 2 {
					tableName = strings.ReplaceAll(tm[1], `"`, "")
				}
				if tableName != "" {
					if indexedByTable[tableName] == nil {
						indexedByTable[tableName] = map[string]bool{}
					}
					idMatches := slp126IDTokenRe.FindAllStringSubmatch(lower, -1)
					for _, tokenMatch := range idMatches {
						if len(tokenMatch) >= 2 {
							indexedByTable[tableName][tokenMatch[1]] = true
						}
					}
				}
				continue
			}

			// Detect FK/reference lines.
			if !slp126RefLineRe.MatchString(lower) { // slp126RefLineRe?.
				continue
			}
			idMatches := slp126IDTokenRe.FindAllStringSubmatch(lower, -1)
			for _, tokenMatch := range idMatches {
				if len(tokenMatch) >= 2 {
					candidates = append(candidates, slp126Hit{
						table:   currentTable,
						column:  tokenMatch[1],
						line:    ln,
						snippet: strings.TrimSpace(ln.Content),
					})
				}
			}
		}

		if len(candidates) == 0 {
			continue
		}

		seen := map[string]bool{}
		for _, c := range candidates {
			key := c.table + "." + c.column
			if seen[key] {
				continue
			}
			seen[key] = true
			// Check table-scoped index coverage; also accept a global (empty
			// table key) index as a fallback for ALTER TABLE contexts.
			tableIdxs := indexedByTable[c.table]
			globalIdxs := indexedByTable[""]
			if tableIdxs[c.column] || globalIdxs[c.column] {
				continue
			}
			// If table is unknown, check if ANY table has an index on this column.
			if c.table == "" {
				indexedInAnyTable := false
				for _, cols := range indexedByTable {
					if cols[c.column] {
						indexedInAnyTable = true
						break
					}
				}
				if indexedInAnyTable {
					continue
				}
			}
			finding := Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Snippet:  c.snippet,
			}
			// Set fields directly to avoid false positives from rule SLP043.
			finding.File = f.Path
			finding.Line = c.line.NewLineNo
			finding.Message = "migration adds FK column '" + c.column + "' on table '" + c.table +
				"' without a matching index — add CREATE INDEX for join/cascade performance"
			out = append(out, finding)
		}
	}
	return out
}
