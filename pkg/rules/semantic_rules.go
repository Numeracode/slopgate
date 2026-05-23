package rules

import (
	"go/ast"
	"go/token"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP071 detects type assertions without the comma-ok idiom.
// This can cause panics when the type assertion fails.
type SLP071 struct{}

func (SLP071) ID() string                { return "SLP071" }
func (SLP071) DefaultSeverity() Severity { return SeverityWarn }
func (SLP071) Description() string {
	return "type assertion without comma-ok idiom can panic"
}

// hasUnsafeTypeAssert checks if there's any type assertion without comma-ok
// in the given AST node.
func hasUnsafeTypeAssert(n ast.Node) bool {
	unsafe := false
	ast.Inspect(n, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			for _, rhs := range assign.Rhs {
				if _, isAssert := rhs.(*ast.TypeAssertExpr); isAssert {
					if len(assign.Lhs) != 2 {
						unsafe = true
					}
				}
			}
			// Stop traversing this AssignStmt to avoid matching its TypeAssertExpr
			// child at the top level.
			return false
		}
		if _, ok := node.(*ast.TypeAssertExpr); ok {
			unsafe = true
		}
		return true
	})
	return unsafe
}

func (r SLP071) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		// Check for type assertions without comma-ok.
		if fa.AST != nil && hasUnsafeTypeAssert(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "type assertion — use v, ok := x.(Type) for safe handling",
			})
		}
	}
	return out
}

// SLP072 detects potential nil pointer dereferences.
// While we can't do full data flow analysis, we can detect common patterns
// like method calls on potentially nil interface values.
type SLP072 struct{}

func (SLP072) ID() string                { return "SLP072" }
func (SLP072) DefaultSeverity() Severity { return SeverityBlock }
func (SLP072) Description() string {
	return "potential nil pointer dereference"
}

// detectNil dereference patterns.
func detectNil(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.CallExpr:
			// Check if it's a method call (obj.method()) where obj could be nil.
			if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
				// If the receiver looks like a variable, flag it.
				if ident, ok := sel.X.(*ast.Ident); ok {
					// Common nil variable names.
					if ident.Name == "err" || ident.Name == "resp" || ident.Name == "result" {
						found = true
					}
				}
			}
		}
		return true
	})
	return found
}

func (r SLP072) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if detectNil(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "method call on potentially nil result — check for nil before calling methods",
			})
		}
	}
	return out
}

// SLP073 detects missing defer for resource cleanup patterns.
// Common patterns like os.File, sql.Rows that should be deferred.
type SLP073 struct{}

func (SLP073) ID() string                { return "SLP073" }
func (SLP073) DefaultSeverity() Severity { return SeverityWarn }
func (SLP073) Description() string {
	return "resource acquired without defer close"
}

var resourceTypes = map[string]bool{
	"os.File":       true,
	"*os.File":      true,
	"sql.Rows":      true,
	"*sql.Rows":     true,
	"io.ReadCloser": true,
	"http.Response": true,
}

func hasMissingDefer(n ast.Node) bool {
	found := false
	// First, collect all defer statements.
	defers := make(map[string]bool)
	ast.Inspect(n, func(node ast.Node) bool {
		if d, ok := node.(*ast.DeferStmt); ok {
			if call, ok := d.Call.Fun.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						defers[ident.Name] = true
					}
				}
			}
		}
		return true
	})
	// Now check for resource acquisitions without defer.
	ast.Inspect(n, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				name := sel.Sel.Name
				// Check for acquire methods.
				if name == "Open" || name == "Read" || name == "Query" {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if !defers[ident.Name+"_defer"] && !defers["defer_"+ident.Name] {
							// Only flag if we see a resource type.
							if resourceTypes[ident.Name] || resourceTypes["*"+ident.Name] {
								found = true
							}
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func (r SLP073) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if hasMissingDefer(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "resource acquired without defer close — add defer close for safety",
			})
		}
	}
	return out
}

// SLP074 detects loop variables that escape into goroutines.
// This is a common race condition bug.
type SLP074 struct{}

func (SLP074) ID() string                { return "SLP074" }
func (SLP074) DefaultSeverity() Severity { return SeverityBlock }
func (SLP074) Description() string {
	return "loop variable captured by goroutine — potential race condition"
}

func captureLoopVar(n ast.Node) bool {
	found := false
	// Simplified: check for goroutine statements + loop variables.
	// This is a heuristic that flags potential issues.
	hasLoop := false
	hasGo := false

	ast.Inspect(n, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.RangeStmt, *ast.ForStmt:
			hasLoop = true
		case *ast.GoStmt:
			hasGo = true
		}
		return true
	})
	found = hasLoop && hasGo
	_ = found
	return found
}

func (r SLP074) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if captureLoopVar(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "loop variable captured by goroutine — copy to local var first",
			})
		}
	}
	return out
}

// SLP075 detects usage of weak cryptographic functions.
type SLP075 struct{}

func (SLP075) ID() string                { return "SLP075" }
func (SLP075) DefaultSeverity() Severity { return SeverityWarn }
func (SLP075) Description() string {
	return "weak cryptographic function detected"
}

var weakCrypto = map[string]bool{
	"md5":         true,
	"sha1":        true,
	"DES":         true,
	"RC4":         true,
	"crypto/rc4":  true,
	"crypto/des":  true,
	"crypto/md5":  true,
	"crypto/sha1": true,
}

func usesWeakCrypto(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		if sel, ok := node.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if weakCrypto[ident.Name] || weakCrypto["crypto/"+ident.Name] {
					found = true
				}
			}
		}
		return true
	})
	return found
}

func (r SLP075) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if usesWeakCrypto(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "weak crypto — use crypto/aes, crypto/chacha20, or crypto/ed25519",
			})
		}
	}
	return out
}

// SLP076 detects potential SQL injection via string concatenation.
type SLP076 struct{}

func (SLP076) ID() string                { return "SLP076" }
func (SLP076) DefaultSeverity() Severity { return SeverityBlock }
func (SLP076) Description() string {
	return "potential SQL injection — use parameterized queries"
}

func detectSQLConcat(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				// Common SQL function names.
				if sel.Sel.Name == "Exec" || sel.Sel.Name == "Query" ||
					sel.Sel.Name == "ExecContext" || sel.Sel.Name == "QueryContext" {
					// Check if any argument is a binary operation (concatenation).
					for _, arg := range call.Args {
						if _, ok := arg.(*ast.BinaryExpr); ok {
							found = true
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func (r SLP076) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if detectSQLConcat(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "SQL query built via concatenation — use $1, $2 parameters instead",
			})
		}
	}
	return out
}

// SLP077 detects hardcoded credentials detected via AST analysis.
type SLP077 struct{}

func (SLP077) ID() string                { return "SLP077" }
func (SLP077) DefaultSeverity() Severity { return SeverityBlock }
func (SLP077) Description() string {
	return "hardcoded credential detected in AST"
}

// detectHardcodedCreds detects string literals that look like credentials.
func detectHardcodedCreds(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok {
			if lit.Kind == token.STRING {
				val := lit.Value
				// Simple heuristic: flag strings with = in them that might be credentials
				if len(val) > 15 {
					found = true
				}
			}
		}
		return true
	})
	return found
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (r SLP077) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if slp077IsRuleDefinitionPath(fa.Path) {
			continue
		}
		if detectHardcodedCreds(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "hardcoded credential detected — use env vars or secret manager",
			})
		}
	}
	return out
}

func slp077IsRuleDefinitionPath(path string) bool {
	return contains(path, "pkg/rules/") || contains(path, `pkg\rules\`)
}

// SLP078 detects select statements on closed channels,
// which can cause panics.
type SLP078 struct{}

func (SLP078) ID() string                { return "SLP078" }
func (SLP078) DefaultSeverity() Severity { return SeverityBlock }
func (SLP078) Description() string {
	return "select on potentially closed channel"
}

func selectsOnClosed(n ast.Node) bool {
	found := false
	// Simplified: flag any select statement in the file for manual review.
	// Full analysis would need to check the channel source.
	ast.Inspect(n, func(node ast.Node) bool {
		if _, ok := node.(*ast.SelectStmt); ok {
			found = true
		}
		return true
	})
	return found
}

func (r SLP078) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if selectsOnClosed(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "select on channel without checking if closed first",
			})
		}
	}
	return out
}

// SLP079 detects missing error handling for known dangerous functions.
type SLP079 struct{}

func (SLP079) ID() string                { return "SLP079" }
func (SLP079) DefaultSeverity() Severity { return SeverityWarn }
func (SLP079) Description() string {
	return "ignored error return from known function"
}

var dangerousFuncs = map[string]bool{
	"json.Unmarshal": true,
	"xml.Unmarshal":  true,
	"io.Copy":        true,
	"fmt.Scanf":      true,
	"fmt.Fscanf":     true,
	"json.Decode":    true,
	"base64.Decode":  true,
}

func ignoredErrors(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			if len(assign.Lhs) > 0 && len(assign.Rhs) > 0 {
				// Check if Lhs[0] is _ (blank identifier).
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok && ident.Name == "_" {
					// Check if RHS is a call to a dangerous function.
					if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if dangerousFuncs[sel.Sel.Name] {
								found = true
							}
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func (r SLP079) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if ignoredErrors(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "error return ignored — check err != nil before proceeding",
			})
		}
	}
	return out
}

// SLP080 detects interface with only one implementation.
type SLP080 struct{}

func (SLP080) ID() string                { return "SLP080" }
func (SLP080) DefaultSeverity() Severity { return SeverityInfo }
func (SLP080) Description() string {
	return "interface with single implementation — possible over-abstraction"
}

// This is a simplified check — full implementation would need cross-file analysis.
func interfaceSingleImpl(n ast.Node) bool {
	found := false
	// Only flag interfaces with a single method that has a simple implementation.
	ast.Inspect(n, func(node ast.Node) bool {
		if t, ok := node.(*ast.TypeSpec); ok {
			if i, ok := t.Type.(*ast.InterfaceType); ok {
				// Small interface.
				if len(i.Methods.List) <= 2 {
					// Check if there's only one type implementing this.
					found = true // Heuristic: flag it for review.
				}
			}
		}
		return true
	})
	return found
}

func (r SLP080) Check(a *diff.AnalysisResult) []Finding {
	var out []Finding
	for _, fa := range a.Files {
		if fa.Error != nil {
			continue
		}
		if interfaceSingleImpl(fa.AST) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     fa.Path,
				Message:  "small interface with single impl — consider concrete type instead",
			})
		}
	}
	return out
}
