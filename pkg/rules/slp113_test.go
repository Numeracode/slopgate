package rules

import (
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP113_FiresOnGoSourceWithoutTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
new file mode 100644
--- /dev/null
+++ b/handler.go
@@ -0,0 +1,5 @@
+package api
+
+func Handler() {}
`)
	got := SLP113{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for .go source without test")
	}
}

func TestSLP113_NoFireOnGoSourceWithTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
new file mode 100644
--- /dev/null
+++ b/handler.go
@@ -0,0 +1,5 @@
+package api
+
+func Handler() {}
diff --git a/handler_test.go b/handler_test.go
new file mode 100644
--- /dev/null
+++ b/handler_test.go
@@ -0,0 +1,5 @@
+package api
+
+func TestHandler(t *testing.T) {}
`)
	got := SLP113{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when test present, got %d", len(got))
	}
}

func TestSLP113_NoFireWhenMatchingTestIsIgnoredFromScan(t *testing.T) {
	d := parseDiff(t, `diff --git a/pkg/rules/slp130.go b/pkg/rules/slp130.go
--- a/pkg/rules/slp130.go
+++ b/pkg/rules/slp130.go
@@ -1,1 +1,3 @@
+func (SLP130) Description() string { return "updated" }
diff --git a/pkg/rules/slp130_test.go b/pkg/rules/slp130_test.go
--- a/pkg/rules/slp130_test.go
+++ b/pkg/rules/slp130_test.go
@@ -1,1 +1,3 @@
+func TestSLP130_Updated(t *testing.T) {}
`)
	filtered := diff.FilterIgnored(d, []string{"pkg/rules/slp*_test.go"})
	got := SLP113{}.Check(filtered)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when ignored test file still exists in diff metadata, got %d", len(got))
	}
}

func TestSLP113_NoFireOnTestFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler_test.go b/handler_test.go
new file mode 100644
--- /dev/null
+++ b/handler_test.go
@@ -0,0 +1,5 @@
+package api
+
+func TestHandler(t *testing.T) {}
`)
	got := SLP113{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for test file, got %d", len(got))
	}
}

func TestSLP113_Description(t *testing.T) {
	r := SLP113{}
	if r.ID() != "SLP113" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
