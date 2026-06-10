package rules

import (
	"testing"
)

func TestSLP222_FlagsWmicOutputWithoutDecode(t *testing.T) {
	d := parseDiff(t, `diff --git a/snapnative.go b/snapnative.go
--- a/snapnative.go
+++ b/snapnative.go
@@ -1,3 +1,6 @@
+func getProcesses() {
+	out, _ := exec.Command("wmic", "process", "list", "brief").Output()
+	if !utf8.Valid(out) {
+		return
+	}
+}
`)
	findings := SLP222{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestSLP222_NoFlagWithBOMDecode(t *testing.T) {
	d := parseDiff(t, `diff --git a/snapnative.go b/snapnative.go
--- a/snapnative.go
+++ b/snapnative.go
@@ -1,3 +1,10 @@
+func getProcesses() {
+	out, _ := exec.Command("wmic", "process", "list", "brief").Output()
+	// Decode UTF-16 BOM
+	if len(out) >= 3 && out[0] == 0xEF && out[1] == 0xBB && out[2] == 0xBF {
+		out = out[3:]
+	}
+	if !utf8.Valid(out) {
+		return
+	}
+}
`)
	findings := SLP222{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when BOM handled, got %d", len(findings))
	}
}

func TestSLP222_NoFlagWithoutWmic(t *testing.T) {
	d := parseDiff(t, `diff --git a/other.go b/other.go
--- a/other.go
+++ b/other.go
@@ -1,3 +1,6 @@
+func runLs() {
+	out, _ := exec.Command("ls").Output()
+	if !utf8.Valid(out) {
+		return
+	}
+}
`)
	findings := SLP222{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings without wmic, got %d", len(findings))
	}
}
