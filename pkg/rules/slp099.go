package rules

import (
	"path"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP099 detects when a response struct/type field is added, renamed, or
// retyped in a non-test file without corresponding test file changes in
// the same diff. This is a common AI slop pattern: the agent changes a
// response shape but doesn't update the tests, causing test drift.
type SLP099 struct{}

func (SLP099) ID() string                { return "SLP099" }
func (SLP099) DefaultSeverity() Severity { return SeverityWarn }
func (SLP099) Description() string {
	return "response field changed without test update — tests may be stale"
}

var slp099GoStructField = regexp.MustCompile(
	`^\s*(?:` +
		`[A-Z]\w*\s+(?:\*\[\]|\[\]\*|\[\]|\*)?\w+(?:\.\w+)?` + // Exported field (uppercase)
		`|\w+\s+(?:\*\[\]|\[\]\*|\[\]|\*)\w+(?:\.\w+)?` + // Any field with pointer/slice type
		`|[A-Z]\w*\s+\w+\.\w+` + // Exported field with package-qualified type
		`)(?:\s+` + "`" + `[^` + "`" + `]*` + "`" + `)?(?:\s*//.*)?$` + // Optional struct tag
		`|^\s*\w+\s+(?:\*\[\]|\[\]\*|\[\]|\*)?\w+(?:\.\w+)?\s+` + "`" + `[^` + "`" + `]*` + "`" + `(?:\s*//.*)?$`) // Lowercase field with struct tag

var slp099TSInterfaceProp = regexp.MustCompile(`(?i)^(?:readonly\s+)?\w+(?:\?)?:\s*(?:string|number|boolean|Date|\[\]\w+|\w+\[\])[;,]?(?:\s*//.*)?$`)

// slp099PythonField matches Python dataclass/Pydantic model field definitions.
// Covers: field: Type, field: Type = default, field: Optional[Type], field: list[Type]
var slp099PythonField = regexp.MustCompile(
	`^\s*[a-z_]\w*\s*:\s*(?:` +
		`(?:Optional|List|Dict|Set|Tuple|Any|str|int|float|bool|bytes|datetime)` +
		`|\w+` +
		`)(?:\[.*?\])?(?:\s*=\s*.+)?(?:\s*#.*)?$`,
)

var slp099ResponseKeywords = map[string]struct{}{
	"response": {},
	"dto":      {},
	"output":   {},
	"result":   {},
	"payload":  {},
	"body":     {},
	"reply":    {},
	"envelope": {},
	"wrapper":  {},
}

var slp099IgnoredTrailingTokens = map[string]struct{}{
	"model":  {},
	"schema": {},
	"type":   {},
	"types":  {},
	"config": {},
	"util":   {},
	"helper": {},
}

// slp099UtilitySuffixes are filename suffixes that indicate a utility/helper
// file rather than an API response type, even if the stem contains a response
// keyword (e.g., result_cache.ts, response_util.go).
var slp099UtilitySuffixes = []string{
	"_cache", ".cache",
	"_util", ".util",
	"_helper", ".helper",
	"_config", ".config",
	"_factory", ".factory",
	"_builder", ".builder",
}

var slp099CamelBoundaryLowerToUpper = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var slp099CamelBoundaryAcronym = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
var slp099NonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)
var slp099VersionToken = regexp.MustCompile(`^v?\d+$`)

func hasResponseKeyword(name string) bool {
	// Reject utility/helper files even if they contain a response keyword.
	lower := strings.ToLower(name)
	for _, suffix := range slp099UtilitySuffixes {
		if strings.HasSuffix(strings.TrimSuffix(lower, path.Ext(lower)), suffix) ||
			strings.HasSuffix(lower, suffix+path.Ext(lower)) {
			return false
		}
	}
	tokens := slp099FilenameTokens(name)
	for i := len(tokens) - 1; i >= 0; i-- {
		tok := tokens[i]
		if _, ok := slp099IgnoredTrailingTokens[tok]; ok {
			continue
		}
		if slp099VersionToken.MatchString(tok) {
			continue
		}
		if _, ok := slp099ResponseKeywords[tok]; ok {
			return true
		}
		return false
	}
	return false
}

func matchesSlp099FieldLine(filePath, content string) bool {
	if isGoFile(filePath) {
		return slp099GoStructField.MatchString(content)
	}
	if isJSOrTSFile(filePath) {
		return slp099TSInterfaceProp.MatchString(content)
	}
	if isPythonFile(filePath) {
		return slp099PythonField.MatchString(content)
	}
	return false
}

// slp099TrimKnownExt strips known multi-part extensions (e.g., .d.ts) before
// falling back to path.Ext so that stems like "response.d.ts" → "response".
func slp099TrimKnownExt(name string) string {
	if strings.HasSuffix(name, ".d.ts") {
		return strings.TrimSuffix(name, ".d.ts")
	}
	return strings.TrimSuffix(name, path.Ext(name))
}

func slp099FilenameTokens(name string) []string {
	base := path.Base(name)
	stem := slp099TrimKnownExt(base)
	stem = slp099CamelBoundaryAcronym.ReplaceAllString(stem, `$1 $2`)
	stem = slp099CamelBoundaryLowerToUpper.ReplaceAllString(stem, `$1 $2`)
	stem = slp099NonAlnum.ReplaceAllString(stem, " ")
	return strings.Fields(strings.ToLower(stem))
}

func (r SLP099) Check(d *diff.Diff) []Finding {
	var out []Finding
	changedFiles := make(map[string]bool)
	changedTestFiles := make(map[string]bool)

	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if isTestFile(f.Path) {
			changedTestFiles[f.Path] = true
			continue
		}
		if !isGoFile(f.Path) && !isJSOrTSFile(f.Path) && !isPythonFile(f.Path) {
			continue
		}

		// Check both added and deleted lines to catch field additions, removals, and renames
		for _, ln := range f.AddedLines() {
			content := strings.TrimSpace(ln.Content)
			if matchesSlp099FieldLine(f.Path, content) {
				if hasResponseKeyword(f.Path) {
					changedFiles[f.Path] = true
				}
			}
		}

		// Also flag on field removals (response structure changed)
		for _, ln := range f.DeletedLines() {
			content := strings.TrimSpace(ln.Content)
			if matchesSlp099FieldLine(f.Path, content) {
				if hasResponseKeyword(f.Path) {
					changedFiles[f.Path] = true
				}
			}
		}
	}

	for _, f := range d.Files {
		if !changedFiles[f.Path] {
			continue
		}
		if testMatchesResponse(f.Path, changedTestFiles) {
			continue
		}

		// Report both added and deleted field changes
		for _, ln := range f.AddedLines() {
			content := strings.TrimSpace(ln.Content)
			if matchesSlp099FieldLine(f.Path, content) {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "response field added/changed without test update — verify tests still match",
					Snippet:  content,
				})
			}
		}

		for _, ln := range f.DeletedLines() {
			content := strings.TrimSpace(ln.Content)
			if matchesSlp099FieldLine(f.Path, content) {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.OldLineNo,
					Message:  "response field removed/changed without test update — verify tests still match",
					Snippet:  content,
				})
			}
		}
	}
	return out
}

func slp099NormalizeStem(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func testMatchesResponse(respPath string, testFiles map[string]bool) bool {
	if len(testFiles) == 0 {
		return false
	}
	respStem := slp099FileStem(respPath)
	normRespStem := slp099NormalizeStem(respStem)
	respDir := path.Dir(respPath)

	for tf := range testFiles {
		if slp099NormalizeStem(slp099FileStem(tf)) != normRespStem {
			continue
		}
		if slp099RelatedDir(respDir, path.Dir(tf)) {
			return true
		}
	}
	return false
}

func slp099FileStem(filePath string) string {
	base := path.Base(filePath)
	stem := slp099TrimKnownExt(base)
	switch {
	case strings.HasSuffix(stem, "_test"):
		return strings.TrimSuffix(stem, "_test")
	case strings.HasSuffix(stem, ".test"):
		return strings.TrimSuffix(stem, ".test")
	case strings.HasSuffix(stem, ".spec"):
		return strings.TrimSuffix(stem, ".spec")
	default:
		return stem
	}
}

func slp099RelatedDir(respDir, testDir string) bool {
	if respDir == testDir {
		return true
	}
	if path.Dir(testDir) == respDir || path.Dir(respDir) == testDir {
		return true
	}
	// Parallel test directories: src/foo → tests/foo, src/foo → test/foo
	for _, testRoot := range []string{"tests", "test", "__tests__", "specs"} {
		if strings.HasPrefix(testDir, testRoot+"/") || testDir == testRoot {
			remainder := strings.TrimPrefix(testDir, testRoot)
			remainder = strings.TrimPrefix(remainder, "/")
			if remainder == "" || respDir == remainder || strings.HasSuffix(respDir, "/"+remainder) {
				return true
			}
		}
	}
	return false
}
