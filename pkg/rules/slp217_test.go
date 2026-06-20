package rules

import (
	"testing"
)

func TestSLP217GoExportedFunctionMissingCheck(t *testing.T) {
	d := parseDiff(t, `diff --git a/backup.go b/backup.go
--- a/backup.go
+++ b/backup.go
@@ -1,5 +1,8 @@
 package backup
+
+func Backup(sourceRoot, remoteDest string) (*Result, error) {
+    stagingDir := filepath.Join(remoteDest, "staging")
+    return nil, nil
+}
`)
	want := 2
	got := len(SLP217{}.Check(d))
	if got != want {
		t.Fatalf("expected %d findings, got %d", want, got)
	}
}

func TestSLP217GoUnexportedFunctionNotFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/backup.go b/backup.go
--- a/backup.go
+++ b/backup.go
@@ -1,5 +1,8 @@
 package backup
+
+func backup(sourceRoot, remoteDest string) (*Result, error) {
+    stagingDir := filepath.Join(remoteDest, "staging")
+    return nil, nil
+}
`)
	got := len(SLP217{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for unexported helper, got %d", got)
	}
}

func TestSLP217GoMethodWithValidationOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/backup.go b/backup.go
--- a/backup.go
+++ b/backup.go
@@ -1,5 +1,10 @@
 package backup
+
+func Backup(sourceRoot, remoteDest string) (*Result, error) {
+    if sourceRoot == "" {
+        return nil, fmt.Errorf("source root required")
+    }
+    return nil, nil
+}
`)
	got := len(SLP217{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding (remoteDest unchecked), got %d", got)
	}
}

func TestSLP217JSExportedFunctionMissingCheck(t *testing.T) {
	d := parseDiff(t, `diff --git a/lib/backup.js b/lib/backup.js
--- a/lib/backup.js
+++ b/lib/backup.js
@@ -1,5 +1,8 @@
-export function runBackup(sourceRoot, remoteDest) {
+export function runBackup(sourceRoot, remoteDest) {
+    const stagingDir = path.join(remoteDest, 'staging');
+    return stagingDir;
 }
`)
	want := 2
	got := len(SLP217{}.Check(d))
	if got != want {
		t.Fatalf("expected %d findings, got %d", want, got)
	}
}

func TestSLP217JSUnexportedFunctionNotFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/lib/backup.js b/lib/backup.js
--- a/lib/backup.js
+++ b/lib/backup.js
@@ -1,5 +1,8 @@
-function runBackup(sourceRoot, remoteDest) {
+function runBackup(sourceRoot, remoteDest) {
+    const stagingDir = path.join(remoteDest, 'staging');
+    return stagingDir;
 }
`)
	got := len(SLP217{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for unexported JS helper, got %d", got)
	}
}

func TestSLP217JSValidationTruthyOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/lib/backup.js b/lib/backup.js
--- a/lib/backup.js
+++ b/lib/backup.js
@@ -1,5 +1,8 @@
+export function runBackup(sourceRoot) {
+    if (!sourceRoot) return;
+    return sourceRoot;
+}
`)
	got := len(SLP217{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings when !sourceRoot validation present, got %d", got)
	}
}

func TestSLP217ModuleExportArrowFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/lib/backup.js b/lib/backup.js
--- a/lib/backup.js
+++ b/lib/backup.js
@@ -1,5 +1,8 @@
-module.exports.runBackup = (sourceRoot, remoteDest) => {
+module.exports.runBackup = (sourceRoot, remoteDest) => {
+    const stagingDir = path.join(remoteDest, 'staging');
+    return stagingDir;
+}
`)
	want := 2
	got := len(SLP217{}.Check(d))
	if got != want {
		t.Fatalf("expected %d findings for module.exports arrow, got %d", want, got)
	}
}
