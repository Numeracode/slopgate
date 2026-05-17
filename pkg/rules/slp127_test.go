package rules

import (
	"strings"
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP127_FiresWhenRuleChangesWithoutTestDiff(t *testing.T) {
	d := parseDiff(t, `diff --git a/pkg/rules/slp130.go b/pkg/rules/slp130.go
--- a/pkg/rules/slp130.go
+++ b/pkg/rules/slp130.go
@@ -1,1 +1,3 @@
+func (SLP130) Description() string { return "updated" }
`)
	got := SLP127{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding when rule implementation changes without test diff")
	}
	if !strings.Contains(got[0].Message, "pkg/rules/slp130_test.go") {
		t.Fatalf("expected message to mention matching test file, got %q", got[0].Message)
	}
}

func TestSLP127_NoFireWhenRuleAndTestBothChange(t *testing.T) {
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
	got := SLP127{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when rule and test both change, got %d", len(got))
	}
}

func TestSLP127_NoFireWhenMatchingTestIsIgnoredFromScan(t *testing.T) {
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
	got := SLP127{}.Check(filtered)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when ignored test diff still exists in metadata, got %d", len(got))
	}
}

func TestSLP127_Description(t *testing.T) {
	r := SLP127{}
	if r.ID() != "SLP127" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
