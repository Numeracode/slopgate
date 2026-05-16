package rules

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP098 flags new API routes or handlers added without any corresponding
// test changes in the same diff. This is a common AI slop pattern: adding
// route handlers without tests.
type SLP098 struct{}

func (SLP098) ID() string                { return "SLP098" }
func (SLP098) DefaultSeverity() Severity { return SeverityWarn }
func (SLP098) Description() string {
	return "new route/handler added without test — add a test for the new endpoint"
}

// slp098RoutePatterns matches explicit route registration calls across
// frameworks (Express, Fastify, Koa, Go, Spring, Flask, FastAPI, etc.).
var slp098RoutePatterns = []*regexp.Regexp{
	// Express.js and general Node.js routers
	regexp.MustCompile(`(?i)(?:app|router|r)\.(?:get|post|put|delete|patch|head|options|use|all)\s*\(`),
	// Express.js route parameter handlers
	regexp.MustCompile(`(?i)(?:app|router|r)\.param\s*\(`),
	// Express.js static file serving
	regexp.MustCompile(`(?i)(?:app|router|r)\.static\s*\(`),
	// Fastify
	regexp.MustCompile(`(?i)(?:fastify|server)\.(?:get|post|put|delete|patch|head|options|route)\s*\(`),
	// Koa
	regexp.MustCompile(`(?i)(?:app|router)\.(?:get|post|put|delete|patch|head|options|use)\s*\(`),
	// Hapi
	regexp.MustCompile(`(?i)server\.route\s*\(\s*\{`),
	// Next.js API route handlers - function declaration form (HTTP method names only)
	regexp.MustCompile(`(?i)export\s+(?:async\s+)?function\s+(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(`),
	// Next.js API route handlers - default function (export default function handler)
	regexp.MustCompile(`(?i)export\s+default\s+(?:async\s+)?function\s+handler\s*\(`),
	// Next.js API route handlers - assignment form (const/let/var with HTTP method names)
	regexp.MustCompile(`(?i)export\s+(?:const|let|var)\s+(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|handler)\s*=`),
	// Java/Spring annotations
	regexp.MustCompile(`(?i)@(?:Get|Post|Put|Delete|Patch|RequestMapping)\s*\(`),
	// Go HTTP handlers
	regexp.MustCompile(`(?i)\.(?:HandleFunc|Handle)\s*\(`),
	regexp.MustCompile(`(?i)(?:mux|router)\.(?:HandleFunc|Handle|NewRoute)\s*\(`),
	// Go Gin
	regexp.MustCompile(`(?i)(?:r|engine|gin)\.(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any|Group)\s*\(`),
	// Go Echo
	regexp.MustCompile(`(?i)(?:e|echo)\.(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any|Group)\s*\(`),
	// Go Fiber
	regexp.MustCompile(`(?i)(?:app|fiber)\.(?:Get|Post|Put|Delete|Patch|Head|Options|Use|All|Group)\s*\(`),
	// Generic route group/router patterns
	regexp.MustCompile(`(?i)(?:group|route)\s*\(\s*["'\x60]/`),
	// Python/Flask patterns
	regexp.MustCompile(`(?i)@(?:route|app\.route|blueprint\.route)\s*\(`),
	// Python/FastAPI
	regexp.MustCompile(`(?i)@(?:app|router)\.(?:get|post|put|delete|patch|head|options)\s*\(`),
	// Python/Django URL patterns
	regexp.MustCompile(`(?i)(?:path|re_path|url)\s*\(\s*["'\x60]`),
	// Ruby on Rails
	regexp.MustCompile(`(?i)(?:get|post|put|delete|patch|resources|resource)\s+["'\x60]/`),
	// Various handler patterns
	regexp.MustCompile(`(?i)\.(?:AddRoute|MapPath|HandlePath)\s*\(`),
	// tRPC routers
	regexp.MustCompile(`(?i)(?:router|publicProcedure|protectedProcedure)\.`),
}

// slp098RouteFileNames matches filenames that are likely route/handler files
// even when the route registration patterns above don't match (e.g., file-based
// routing in Next.js, or config-driven routers).
var slp098RouteFileNames = regexp.MustCompile(`(?i)(?:^|/)(?:routes?|api(?:s)?|endpoints?|handlers?|controllers?|rest)(?:/|[^/]*\.(?:go|ts|tsx|js|jsx|py|java|rb)$)`)

func (r SLP098) Check(d *diff.Diff) []Finding {
	var out []Finding
	routeFiles := make(map[string]bool)
	testFiles := make(map[string]bool)

	// Create a one-time map for file lookup to avoid re-scanning d.Files
	fileByPath := make(map[string]*diff.File)
	for i := range d.Files {
		fileByPath[d.Files[i].Path] = &d.Files[i]
	}

	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if isTestFile(f.Path) {
			if len(f.AddedLines()) > 0 {
				testFiles[f.Path] = true
			}
			continue
		}
		if isDocFile(f.Path) {
			continue
		}

		isRouteFile := false
		for _, ln := range f.AddedLines() {
			content := strings.TrimSpace(ln.Content)
			for _, pat := range slp098RoutePatterns {
				if pat.MatchString(content) {
					isRouteFile = true
					routeFiles[f.Path] = true
					break
				}
			}
			if isRouteFile {
				break
			}
		}
		// Also flag files that look like route/handler files by naming convention
		// (covers file-based routing, config-driven routers, etc.)
		if !isRouteFile && f.IsNew && slp098RouteFileNames.MatchString(f.Path) {
			routeFiles[f.Path] = true
		}
	}

	routePaths := make([]string, 0, len(routeFiles))
	for rf := range routeFiles {
		routePaths = append(routePaths, rf)
	}
	sort.Strings(routePaths)

	for _, rf := range routePaths {
		base := strings.TrimSuffix(rf, path.Ext(rf))

		foundTest := false
		for tf := range testFiles {
			if slp098TestMatches(tf, base) {
				foundTest = true
				break
			}
		}

		if !foundTest {
			f, ok := fileByPath[rf]
			if ok {
				addedLines := f.AddedLines()
				if len(addedLines) == 0 {
					continue
				}
				// Check if any added line matches a route pattern
				hasRouteLine := false
				for _, ln := range addedLines {
					for _, pat := range slp098RoutePatterns {
						if pat.MatchString(ln.Content) {
							out = append(out, Finding{
								RuleID:   r.ID(),
								Severity: r.DefaultSeverity(),
								File:     f.Path,
								Line:     ln.NewLineNo,
								Message:  "new route added without corresponding test changes for this module in this diff",
								Snippet:  ln.Content,
							})
							hasRouteLine = true
							break
						}
					}
					if hasRouteLine {
						break
					}
				}
				// File detected by naming convention — flag on first added line
				if !hasRouteLine && slp098RouteFileNames.MatchString(f.Path) {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     addedLines[0].NewLineNo,
						Message:  "new route/handler file added without corresponding test file",
						Snippet:  addedLines[0].Content,
					})
				}
			}
		}
	}

	return out
}

// slp098TestMatches returns true if testPath is considered related to sourceBase
// (sourceBase is the route file path without extension).
func slp098TestMatches(testPath, sourceBase string) bool {
	testBase := slp098TestTarget(testPath)
	if testBase == sourceBase {
		return true
	}

	if testBase == "" {
		return false
	}

	// Also check if the test path lives under a parallel test/tests directory.
	// Strip common test root prefixes and compare the remainder against the
	// module-relative source path (exact match only — no loose suffix/base checks).
	for _, root := range []string{"tests/", "test/", "src/tests/", "src/test/"} {
		if strings.HasPrefix(testBase, root) {
			remainder := testBase[len(root):]
			// Exact match
			if sourceBase == remainder {
				return true
			}
			// Exact module-relative match: strip a known source root from sourceBase
			for _, srcRoot := range []string{"src", "lib", "app"} {
				prefix := srcRoot + "/"
				if strings.HasPrefix(sourceBase, prefix) {
					rel := strings.TrimPrefix(sourceBase, prefix)
					if rel == remainder {
						return true
					}
				}
			}
		}
	}

	// Handle source files under src/, lib/, app/ — check parallel test(s)/ directory.
	srcBase := path.Base(sourceBase)
	testBaseName := path.Base(testBase)
	if srcBase == testBaseName {
		// Same filename stem — check if directories are parallel (e.g., src/foo vs tests/foo)
		srcDir := path.Dir(sourceBase)
		testDir := path.Dir(testBase)
		for _, root := range []string{"tests", "test", "src/tests", "src/test"} {
			if testDir == root || strings.HasPrefix(testDir, root+"/") {
				remainder := strings.TrimPrefix(testDir, root)
				remainder = strings.TrimPrefix(remainder, "/")
				if remainder == "" || srcDir == remainder || strings.HasSuffix(srcDir, "/"+remainder) {
					return true
				}
			}
		}
		for _, srcRoot := range []string{"src", "lib", "app"} {
			srcReplaced := strings.TrimPrefix(srcDir, srcRoot+"/")
			if srcReplaced == srcDir {
				continue
			}
			for _, testRoot := range []string{"tests", "test"} {
				if testDir == testRoot+"/"+srcReplaced {
					return true
				}
			}
			// Handle src/<module>/test[s] pattern (e.g., src/routes/test vs src/routes)
			for _, testSuffix := range []string{"/test", "/tests"} {
				if testDir == srcRoot+"/"+srcReplaced+testSuffix {
					return true
				}
				// Also handle when testDir has the srcRoot prefix and a test suffix
				if strings.TrimSuffix(testDir, testSuffix) == srcRoot+"/"+srcReplaced {
					return true
				}
			}
		}
	}
	return false
}

func slp098TestTarget(testPath string) string {
	ext := path.Ext(testPath)
	stem := strings.TrimSuffix(testPath, ext)
	switch {
	case strings.HasSuffix(stem, "_test"):
		return strings.TrimSuffix(stem, "_test")
	case strings.HasSuffix(stem, ".test"):
		return strings.TrimSuffix(stem, ".test")
	case strings.HasSuffix(stem, ".spec"):
		return strings.TrimSuffix(stem, ".spec")
	default:
		// Strip "test"/"tests" only when they form the standalone basename
		// or are preceded by a delimiter, to avoid truncating words like "latest".
		base := path.Base(stem)
		lowerBase := strings.ToLower(base)
		// fullLowerBase includes the extension for Java-style suffix checks.
		fullLowerBase := strings.ToLower(path.Base(testPath))
		dir := path.Dir(stem)
		for _, sfx := range []string{"tests", "test"} {
			if lowerBase == sfx {
				return dir
			}
			// Java-style: UserTest.java or UserTests.java (fullLowerBase has extension)
			if strings.HasSuffix(fullLowerBase, sfx+".java") {
				newBase := base[:len(base)-len(sfx)]
				return path.Join(dir, newBase)
			}
			for _, delim := range []string{".", "-"} {
				if strings.HasSuffix(lowerBase, delim+sfx) {
					newBase := base[:len(base)-len(delim+sfx)]
					return path.Join(dir, newBase)
				}
			}
		}
		return stem
	}
}
