package rules

import (
	"path/filepath"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP215 flags diffs that modify API handler or route files while the
// corresponding OpenAPI contract file is untouched — a spec drift smell.
//
// Reviewer pattern (whimsy PRs #1967, #1951, #1952, #1954, #1955): handlers
// gain or change an endpoint but the shipped API spec JSON is not updated
// in the same diff, so client SDKs regenerate against a stale contract.
//
// Heuristic:
//   - If any diff file matches a handler/route path pattern
//   - AND any file in the repo matches an OpenAPI contract path pattern
//     (detected via filename — *.openapi.json, openapi.yaml, swagger.yaml/json)
//   - BUT that contract file is NOT in the diff
//   - → warn once per diff.
//
// We deliberately warn once per diff rather than per handler file so a
// large endpoint refactor does not spam the reporter.
type SLP215 struct{}

func (SLP215) ID() string                { return "SLP215" }
func (SLP215) DefaultSeverity() Severity { return SeverityWarn }
func (SLP215) Description() string {
	return "API handler changed without updating the OpenAPI contract"
}

// handlerPathSignals returns true when the file looks like an HTTP
// handler or route definition.
func handlerPathSignals(path string) bool {
	lower := strings.ToLower(path)
	base := filepath.Base(lower)

	// Go handler files: handlers_*.go, handler_*.go, *_handler.go
	if strings.HasPrefix(base, "handlers_") || strings.HasPrefix(base, "handler_") {
		return true
	}
	// Go handler directories: */handlers/*.go, */handler/*.go
	if strings.Contains(lower, "/handlers/") || strings.Contains(lower, "/handler/") {
		return true
	}
	// Go handler directories: handlers/*.go, handler/*.go
	if strings.Contains(lower, "/handlers/") || strings.Contains(lower, "/handler/") {
		return true
	}
	// JS/TS route files: routes/*.js, routes/*.ts, *_routes.*, *_route.*
	if strings.Contains(lower, "/routes/") || strings.Contains(lower, "/route/") {
		return true
	}
	if strings.HasPrefix(base, "routes.") || strings.HasPrefix(base, "route.") {
		return true
	}
	return false
}

// openapiContractPath returns true when the path names an OpenAPI (or
// Swagger) contract file.
func openapiContractPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, ".openapi.json") ||
		strings.HasSuffix(base, ".openapi.yaml") ||
		strings.HasSuffix(base, ".openapi.yml") ||
		base == "openapi.json" ||
		base == "openapi.yaml" ||
		base == "openapi.yml" ||
		base == "swagger.json" ||
		base == "swagger.yaml" ||
		base == "swagger.yml"
}

func (r SLP215) Check(d *diff.Diff) []Finding {
	handlerFiles, contractChanged := 0, false
	var contractPaths []string // candidates that exist in the repo (from diff only)
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		if handlerPathSignals(f.Path) {
			handlerFiles++
		}
		if openapiContractPath(f.Path) {
			contractPaths = append(contractPaths, f.Path)
			// Only count as "changed" when the file actually has hunks —
			// a placeholder entry (no hunks, not deleted) means the repo
			// contains a contract but didn't modify it in this diff.
			if len(f.Hunks) > 0 || f.IsNew {
				contractChanged = true
			}
		}
	}
	// No handler changes, or contract was touched — nothing to flag.
	if handlerFiles == 0 || contractChanged {
		return nil
	}

	// Without filesystem access we cannot list every contract file in the
	// repo. To avoid a false positive on repos that don't use OpenAPI at
	// all, require the handler file to live near a contract-style path
	// (a /contract/ or /openapi/ or /spec/ directory anywhere in the diff
	// signals the repo has a contract). Otherwise skip.
	// Without filesystem access, the only evidence that the repo uses
	// contracts is what's in the diff. Accept any of:
	//   - a path under a known contract-style directory
	//   - a diff file whose name looks like a contract (even if deleted)
	//   - the handler file living in /api/ or under /internal/ next to
	//     evidence of a contract (heuristic: any "openapi"/"swagger"
	//     path component anywhere in the diff).
	hasContractSignal := false
	for _, f := range d.Files {
		lower := strings.ToLower(f.Path)
		switch {
		case strings.Contains(lower, "/contracts/"):
			hasContractSignal = true
		case strings.Contains(lower, "/openapi/"),
			strings.Contains(lower, "openapi/"):
			hasContractSignal = true
		case strings.Contains(lower, "/specs/"):
			hasContractSignal = true
		case strings.Contains(lower, "/api-spec/"):
			hasContractSignal = true
		case strings.Contains(lower, "/swagger/"):
			hasContractSignal = true
		}
	}
	if len(contractPaths) > 0 {
		hasContractSignal = true
	}
	if !hasContractSignal {
		return nil
	}

	// Report at the first handler file.
	for _, f := range d.Files {
		if handlerPathSignals(f.Path) {
			return []Finding{{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     firstAddLine(f),
				Message:  "handler/route file changed but no OpenAPI contract was updated in this diff — keep spec and implementation in sync",
			}}
		}
	}
	return nil
}

// firstAddLine returns the first added line's line number, falling back to 0.
func firstAddLine(f diff.File) int {
	for _, h := range f.Hunks {
		for _, ln := range h.Lines {
			if ln.Kind == diff.LineAdd {
				return ln.NewLineNo
			}
		}
	}
	return 0
}
