package rules

import (
	"strings"
	"testing"
)

func TestSLP220_FlagsWalkWithoutCtxCheck(t *testing.T) {
	d := parseDiff(t, `diff --git a/walker.go b/walker.go
--- a/walker.go
+++ b/walker.go
@@ -1,3 +1,6 @@
+func walkDir(root string) error {
+	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
+		return nil
+	})
+}
`)
	findings := SLP220{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(strings.ToLower(findings[0].Message), "cancel") {
		t.Errorf("expected message about cancellation, got: %s", findings[0].Message)
	}
}

func TestSLP220_NoFlagWithCtxErrCheck(t *testing.T) {
	d := parseDiff(t, `diff --git a/walker.go b/walker.go
--- a/walker.go
+++ b/walker.go
@@ -1,3 +1,8 @@
+func walkDir(ctx context.Context, root string) error {
+	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
+		if ctx.Err() != nil {
+			return ctx.Err()
+		}
+		return nil
+	})
+}
`)
	findings := SLP220{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when ctx.Err() checked, got %d", len(findings))
	}
}

func TestSLP220_NoFlagWithoutWalk(t *testing.T) {
	d := parseDiff(t, `diff --git a/other.go b/other.go
--- a/other.go
+++ b/other.go
@@ -1,2 +1,4 @@
+func process() {
+	// no walk here
+}
`)
	findings := SLP220{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings without Walk call, got %d", len(findings))
	}
}

func TestSLP220_FlagsWalkDirWithoutCtxCheck(t *testing.T) {
	d := parseDiff(t, `diff --git a/walker.go b/walker.go
--- a/walker.go
+++ b/walker.go
@@ -1,3 +1,6 @@
+func walkDir(root string) error {
+	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
+		return nil
+	})
+}
`)
	findings := SLP220{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for WalkDir, got %d", len(findings))
	}
}
