package rules

import (
	"strings"
	"testing"
)

func TestSLP217_FlagsEmptyPathParamWithoutValidation(t *testing.T) {
	d := parseDiff(t, `diff --git a/connector/stage/backup.go b/connector/stage/backup.go
--- a/connector/stage/backup.go
+++ b/connector/stage/backup.go
@@ -1,0 +1,5 @@
+func Backup(sourceRoot string, remoteDest string) error {
+	// process files
+	return nil
+}
+
`)
	findings := SLP217{}.Check(d)
	if len(findings) < 2 {
		t.Errorf("expected at least 2 findings (one per unvalidated param), got %d", len(findings))
	}
	hasSourceRoot := false
	hasRemoteDest := false
	for _, f := range findings {
		if strings.Contains(f.Message, "sourceRoot") {
			hasSourceRoot = true
		}
		if strings.Contains(f.Message, "remoteDest") {
			hasRemoteDest = true
		}
	}
	if !hasSourceRoot {
		t.Errorf("expected finding about sourceRoot")
	}
	if !hasRemoteDest {
		t.Errorf("expected finding about remoteDest")
	}
}

func TestSLP217_NoWarningWhenEmptyCheckPresent(t *testing.T) {
	d := parseDiff(t, `diff --git a/connector/stage/backup.go b/connector/stage/backup.go
--- a/connector/stage/backup.go
+++ b/connector/stage/backup.go
@@ -1,0 +1,7 @@
+func Backup(sourceRoot string) error {
+	if sourceRoot == "" {
+		return fmt.Errorf("sourceRoot cannot be empty")
+	}
+	// process files
+	return nil
+}
`)
	findings := SLP217{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when empty check is present, got %d", len(findings))
	}
}

func TestSLP217_NoWarningForNonPathParams(t *testing.T) {
	d := parseDiff(t, `diff --git a/connector/stage/backup.go b/connector/stage/backup.go
--- a/connector/stage/backup.go
+++ b/connector/stage/backup.go
@@ -1,0 +1,4 @@
+func Backup(userId string, count int) error {
+	return nil
+}
+
`)
	findings := SLP217{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-path parameters, got %d", len(findings))
	}
}

func TestSLP217_JS_ArrowFunction(t *testing.T) {
	d := parseDiff(t, `diff --git a/lib/utils.ts b/lib/utils.ts
--- a/lib/utils.ts
+++ b/lib/utils.ts
@@ -1,0 +1,3 @@
+const processDir = (dirPath: string) => {
+  return dirPath.toUpperCase()
+}
`)
	findings := SLP217{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for JS arrow function, got %d", len(findings))
	}
}
