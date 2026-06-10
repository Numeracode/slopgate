package rules

import (
	"strings"
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP215_FlagsHandlerChangeWithoutContract(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/handlers/user.go b/internal/handlers/user.go
--- a/internal/handlers/user.go
+++ b/internal/handlers/user.go
@@ -1,2 +1,3 @@
 package handlers
+func GetUser(w http.ResponseWriter, r *http.Request) {}
`)
	// Add a non-contract file living under a contract-style directory so
	// the heuristic knows the repo has contracts (without marking the
	// contract itself as changed — that would suppress the warning).
	d.Files = append(d.Files, diff.File{
		Path: "openapi/README.md",
	})

	findings := SLP215{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "OpenAPI contract") {
		t.Errorf("expected message about OpenAPI contract, got: %s", findings[0].Message)
	}
}

func TestSLP215_NoWarningWhenContractUpdated(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/handlers/user.go b/internal/handlers/user.go
--- a/internal/handlers/user.go
+++ b/internal/handlers/user.go
@@ -1,2 +1,3 @@
 package handlers
+func GetUser(w http.ResponseWriter, r *http.Request) {}
diff --git a/api/openapi.json b/api/openapi.json
--- a/api/openapi.json
+++ b/api/openapi.json
@@ -1,0 +1,5 @@
+{
+  "paths": {
+    "/users/{id}": {}
+  }
+}
`)
	findings := SLP215{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when contract is updated, got %d", len(findings))
	}
}

func TestSLP215_NoWarningForNonHandlerFiles(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/utils.go b/internal/utils.go
--- a/internal/utils.go
+++ b/internal/utils.go
@@ -1,2 +1,3 @@
 package utils
+func Helper() {}
`)
	findings := SLP215{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-handler file, got %d", len(findings))
	}
}
