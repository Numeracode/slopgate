package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP148 detects when different variables representing the same conceptual
// entity use inconsistent naming conventions across modified modules/files.
// This catches patterns like userId vs userID vs user_id for the same concept.
//
// Detection strategy:
//  1. Extract all variable/constant declarations from added lines
//  2. Normalize names (lowercase, strip underscores, etc.)
//  3. Group by semantic similarity (Levenshtein distance, shared prefixes/suffixes)
//  4. Flag groups with multiple naming conventions
//
// Languages: JavaScript, TypeScript, Go, Python
//
// Scope: exported / module-boundary declarations across files. A
// naming inconsistency in a public symbol crosses module lines and is
// worth flagging; a local variable's casing is noise.
type SLP148 struct{}

func (SLP148) ID() string                { return "SLP148" }
func (SLP148) DefaultSeverity() Severity { return SeverityWarn }
func (SLP148) Description() string {
	return "inconsistent naming for the same exported symbol across modules"
}

// These patterns capture the declared name of an exported symbol per
// language: JS/TS `export`, Go capitalised declarations, Python
// module-level public defs/classes.
var (
	slp148JSExport = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|class|const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)`)
	slp148GoFunc   = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Z]\w*)\s*[(\[]`)
	slp148GoType   = regexp.MustCompile(`^\s*(?:type|var|const)\s+([A-Z]\w*)\b`)
	slp148PyDef    = regexp.MustCompile(`^(?:async\s+)?(?:def|class)\s+([A-Za-z]\w*)`)
)

// ignoreList contains common generic names that shouldn't be checked.
var ignoreList = map[string]bool{
	"err":     true,
	"ctx":     true,
	"req":     true,
	"res":     true,
	"data":    true,
	"result":  true,
	"error":   true,
	"message": true,
	"config":  true,
	"options": true,
	"params":  true,
	"body":    true,
	"headers": true,
	"status":  true,
	"id":      true, // too generic - can be many different ids
	"key":     true,
	"value":   true,
	"name":    true,
	"type":    true,
	"time":    true,
	"date":    true,
	"url":     true,
	"path":    true,
	"file":    true,
	"dir":     true,
	"user":    true, // user could be many types
	"item":    true,
	"obj":     true,
	"arr":     true,
	"map":     true,
	"set":     true,
}

// semanticGroups maps common semantic categories.
// Each key is a canonical concept that should have consistent naming.
// Note: "entry" appears in both "record" and "item" groups; the
// semanticGroupKeys slice ensures deterministic resolution.
var semanticGroups = map[string][]string{
	"id":           {"identifier", "uid", "uuid", "guid"},
	"user":         {"user", "account", "profile", "customer", "client"},
	"token":        {"token", "accesstoken", "authtoken", "bearer"},
	"key":          {"key", "apikey", "secret_key"},
	"secret":       {"secret", "apisecret", "password"},
	"config":       {"config", "configuration", "settings", "options"},
	"param":        {"parameter", "param", "arg", "argument"},
	"value":        {"value", "val", "result", "output"},
	"error":        {"error", "err", "failure"},
	"message":      {"message", "msg", "text"},
	"notification": {"notification", "notif", "alert"},
	"record":       {"record", "rec", "entry"},
	"item":         {"item", "entry"},
	"count":        {"count", "total", "num", "number"},
}

// semanticGroupKeys provides a deterministic iteration order for semanticGroups.
// This ensures "entry" always resolves to the same concept regardless of Go
// map iteration order.
var semanticGroupKeys = []string{
	"id", "user", "token", "key", "secret", "config", "param", "value",
	"error", "message", "notification", "record", "item", "count",
}

// normalizeName returns a canonical semantic group key for a variable name.
// It attempts to group true convention variants (e.g., userId, userID, user_id)
// but NOT distinct fields that share a prefix (e.g., userId vs userEmail).
func normalizeName(name string) string {
	lower := strings.ToLower(name)

	// Check each semantic group: exact match only, in deterministic order.
	// Prefix matching is deliberately avoided to prevent collapsing
	// distinct fields like userId and userEmail into the same group.
	for _, concept := range semanticGroupKeys {
		variants := semanticGroups[concept]
		for _, variant := range variants {
			if lower == variant {
				return concept
			}
		}
	}

	// Strip common suffixes and return base, but only if the remaining
	// base is a known semantic group. This prevents "emailId" from
	// normalizing to "email" (unknown) and then matching other "email*" names.
	suffixes := []string{"id", "ids", "uid", "uuid", "key", "token", "url", "path", "file", "dir"}
	base := lower
	for _, suf := range suffixes {
		if strings.HasSuffix(lower, suf) && len(lower) > len(suf) {
			base = strings.TrimSuffix(lower, suf)
			break
		}
	}
	// Remove trailing separators left after suffix stripping (e.g., "user_id" -> "user_" not "user")
	base = strings.TrimRight(base, "_-")

	// Only return the base if it maps to a known semantic group
	if base != lower && len(base) > 0 {
		if _, known := semanticGroups[base]; known {
			return base
		}
	}

	return lower
}

// stripStringsAndComments removes string literals and comments from a line
// to avoid extracting identifiers from within them.
func stripStringsAndComments(line string) string {
	// Remove string literals and comments by scanning character-by-character.
	// This correctly handles // and # inside string literals.
	var result strings.Builder
	inStr := false
	strChar := byte(0)
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inStr {
			if ch == '\\' && i+1 < len(line) {
				i++ // skip escaped char
				continue
			}
			if ch == strChar {
				inStr = false
			}
			continue
		}
		// Check for comment start
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			break // rest is comment
		}
		if ch == '#' {
			break // rest is comment
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			inStr = true
			strChar = ch
			continue
		}
		result.WriteByte(ch)
	}
	return result.String()
}

// slp148ExportedDecls returns the names of exported / module-boundary
// symbols declared on a line, using the declaration syntax of the
// file's language. Strings and comments are stripped first.
func slp148ExportedDecls(content, filePath string) []string {
	cleaned := stripStringsAndComments(content)
	var names []string
	switch {
	case isJSOrTSFile(filePath):
		if m := slp148JSExport.FindStringSubmatch(cleaned); len(m) > 1 {
			names = append(names, m[1])
		}
	case isGoFile(filePath):
		for _, re := range []*regexp.Regexp{slp148GoFunc, slp148GoType} {
			if m := re.FindStringSubmatch(cleaned); len(m) > 1 {
				names = append(names, m[1])
			}
		}
	case isPythonFile(filePath):
		// Module-level only (unindented) and public (no leading _).
		if content != "" && content[0] != ' ' && content[0] != '\t' {
			if m := slp148PyDef.FindStringSubmatch(cleaned); len(m) > 1 && !strings.HasPrefix(m[1], "_") {
				names = append(names, m[1])
			}
		}
	}
	var out []string
	for _, name := range names {
		if len(name) >= 2 && !ignoreList[strings.ToLower(name)] {
			out = append(out, name)
		}
	}
	return out
}

func (r SLP148) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding

	// Collect all added identifiers across all files
	type nameEntry struct {
		name     string
		file     string
		lineNo   int
		normName string
	}
	var allNames []nameEntry

	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		// Get all added lines content
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				line := ln.Content
				// Only exported / module-boundary declarations.
				ids := slp148ExportedDecls(line, f.Path)
				for _, id := range ids {
					norm := normalizeName(id)
					// Only consider normalized forms that aren't empty
					if norm != "" && !ignoreList[strings.ToLower(id)] {
						allNames = append(allNames, nameEntry{
							name:     id,
							file:     f.Path,
							lineNo:   ln.NewLineNo,
							normName: norm,
						})
					}
				}
			}
		}
	}

	// Group by normalized name, then find variant groups
	groups := make(map[string][]nameEntry)
	for _, entry := range allNames {
		groups[entry.normName] = append(groups[entry.normName], entry)
	}

	// For each group with multiple variants, check if they're truly different
	// naming styles for the same concept
	for norm, variants := range groups {
		if len(variants) < 2 {
			continue
		}
		// Find distinct name variants
		variantSet := make(map[string]bool)
		for _, v := range variants {
			variantSet[v.name] = true
		}
		if len(variantSet) < 2 {
			continue // only one distinct name
		}

		// Format message showing variants
		var variantList []string
		for v := range variantSet {
			variantList = append(variantList, v)
		}
		sort.Strings(variantList)
		variantStr := strings.Join(variantList, ", ")

		// Find a representative file/line for the finding
		if len(variants) == 0 {
			continue
		}
		repFile := variants[0].file
		repLine := variants[0].lineNo

		out = append(out, Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			File:     repFile,
			Line:     repLine,
			Message:  "inconsistent naming for '" + norm + "': " + variantStr,
			Snippet:  "consider standardizing to one convention",
		})
	}

	return out
}
