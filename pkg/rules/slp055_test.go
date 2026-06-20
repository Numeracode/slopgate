//go:build ignore_retired

package rules

import (
	"strings"
	"testing"
)

func TestSLP055_FiresOnComplexFuncWithoutComments(t *testing.T) {
	d := parseDiff(t, `diff --git a/a/foo.go b/a/foo.go
--- a/a/foo.go
+++ b/a/foo.go
@@ -1,2 +1,16 @@
 package a

+func Complex() {
+	if a {
+		if b {
+			for i := 0; i < 10; i++ {
+				if c {
+					switch d {
+					}
+				}
+			}
+		}
+	}
+}
`)
	got := SLP055{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "conditionals") {
		t.Errorf("expected message about conditionals, got %q", got[0].Message)
	}
}

func TestSLP055_IgnoresWithComments(t *testing.T) {
	d := parseDiff(t, `diff --git a/a/foo.go b/a/foo.go
--- a/a/foo.go
+++ b/a/foo.go
@@ -1,2 +1,18 @@
 package a

+func Complex() {
+	// handle level one
+	if a {
+		// handle level two
+		if b {
+			for i := 0; i < 10; i++ {
+				if c {
+					switch d {
+					}
+				}
+			}
+		}
+	}
+}
`)
	got := SLP055{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings with comments, got %d: %+v", len(got), got)
	}
}

func TestSLP055_IgnoresSimpleFunc(t *testing.T) {
	d := parseDiff(t, `diff --git a/a/foo.go b/a/foo.go
--- a/a/foo.go
+++ b/a/foo.go
@@ -1,2 +1,5 @@
 package a

+func Simple() {
+	if a {}
+}
`)
	got := SLP055{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for simple func, got %d: %+v", len(got), got)
	}
}

func TestSLP055_Description(t *testing.T) {
	r := SLP055{}
	if r.ID() != "SLP055" {
		t.Errorf("ID = %q", r.ID())
	}
}
