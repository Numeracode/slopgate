package rules

import (
	"path/filepath"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP113 checks for source files changed without a corresponding test update.
type SLP113 struct{}

func (SLP113) ID() string                { return "SLP113" }
func (SLP113) DefaultSeverity() Severity { return SeverityWarn }
func (SLP113) Description() string {
	return "source file changed without corresponding test update — update tests or add a test file"
}

var slp113SourceExtMap = map[string]string{
	".go":   "_test.go",
	".js":   ".test.js",
	".ts":   ".test.ts",
	".tsx":  ".test.tsx",
	".jsx":  ".test.jsx",
	".py":   "_test.py",
	".java": "Test.java",
	".kt":   "Test.kt",
}

func slp113IsTestFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	testSuffix, ok := slp113SourceExtMap[ext]
	if ok && strings.HasSuffix(base, testSuffix) {
		return true
	}
	if strings.Contains(path, ".test.") || strings.Contains(path, "_test.") {
		return true
	}
	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, "/testdata/") || strings.HasSuffix(normalized, "/testdata") ||
		strings.HasPrefix(normalized, "testdata/") || normalized == "testdata" {
		return true
	}
	return false
}

func slp113TestPath(dir, base, testSuffix string) string {
	if dir == "." || dir == "" {
		return base + testSuffix
	}
	return dir + "/" + base + testSuffix
}

func slp113HasTestFile(sourcePath string, allFiles map[string]bool) bool {
	ext := filepath.Ext(sourcePath)
	testSuffix, ok := slp113SourceExtMap[ext]
	if !ok {
		return true
	}

	dir := filepath.Dir(sourcePath)
	base := strings.TrimSuffix(filepath.Base(sourcePath), ext)

	testName := filepath.ToSlash(slp113TestPath(dir, base, testSuffix))
	if allFiles[testName] {
		return true
	}

	testDir := filepath.ToSlash(filepath.Join(dir, "testdata"))
	for f := range allFiles {
		if !strings.HasPrefix(f, testDir+"/") {
			continue
		}
		fileBase := filepath.Base(f)
		stem := strings.TrimSuffix(fileBase, filepath.Ext(fileBase))
		if stem == base {
			return true
		}
		if strings.HasPrefix(stem, base) && len(stem) > len(base) {
			delim := stem[len(base)]
			if delim == '.' || delim == '-' || delim == '_' {
				return true
			}
		}
	}

	return false
}

func slp113ExpectedTestFile(dir, base, ext string) string {
	testSuffix := slp113SourceExtMap[ext]
	switch ext {
	case ".go":
		return slp113TestPath(dir, base, "_test.go")
	case ".py":
		return slp113TestPath(dir, base, "_test.py")
	case ".java":
		return slp113TestPath(dir, base, "Test.java")
	case ".kt":
		return slp113TestPath(dir, base, "Test.kt")
	default:
		return slp113TestPath(dir, base, testSuffix)
	}
}

func (r SLP113) Check(d *diff.Diff) []Finding {
	var out []Finding
	allFiles := make(map[string]bool)

	for _, f := range d.AllFiles() {
		if !f.IsDelete {
			allFiles[filepath.ToSlash(f.Path)] = true
		}
	}

	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}

		ext := filepath.Ext(f.Path)
		if _, ok := slp113SourceExtMap[ext]; !ok {
			continue
		}
		if slp113IsTestFile(f.Path) {
			continue
		}

		if !slp113HasTestFile(f.Path, allFiles) {
			dir := filepath.Dir(f.Path)
			base := strings.TrimSuffix(filepath.Base(f.Path), ext)
			expectedTestFile := slp113ExpectedTestFile(dir, base, ext)
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     1,
				Message:  "source file changed without corresponding test file — expected " + expectedTestFile,
				Snippet:  f.Path,
			})
		}
	}

	return out
}
