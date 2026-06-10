package rules

import (
	"strings"
	"testing"
)

func TestSLP221_FlagsExecCommandWithoutStderr(t *testing.T) {
	d := parseDiff(t, `diff --git a/runner.go b/runner.go
--- a/runner.go
+++ b/runner.go
@@ -1,3 +1,6 @@
+func runBuild() error {
+	cmd := exec.Command("make", "build")
+	return cmd.Run()
+}
`)
	findings := SLP221{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(strings.ToLower(findings[0].Message), "stderr") {
		t.Errorf("expected message about stderr, got: %s", findings[0].Message)
	}
}

func TestSLP221_NoFlagWithStderrPipe(t *testing.T) {
	d := parseDiff(t, `diff --git a/runner.go b/runner.go
--- a/runner.go
+++ b/runner.go
@@ -1,3 +1,8 @@
+func runBuild() error {
+	cmd := exec.Command("make", "build")
+	stderr, _ := cmd.StderrPipe()
+	defer stderr.Close()
+	return cmd.Run()
+}
`)
	findings := SLP221{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when StderrPipe used, got %d", len(findings))
	}
}

func TestSLP221_NoFlagWithStderrAssignment(t *testing.T) {
	d := parseDiff(t, `diff --git a/runner.go b/runner.go
--- a/runner.go
+++ b/runner.go
@@ -1,3 +1,7 @@
+func runBuild() error {
+	cmd := exec.Command("make", "build")
+	cmd.Stderr = os.Stderr
+	return cmd.Run()
+}
`)
	findings := SLP221{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when Stderr assigned, got %d", len(findings))
	}
}

func TestSLP221_FlagsWithContext(t *testing.T) {
	d := parseDiff(t, `diff --git a/runner.go b/runner.go
--- a/runner.go
+++ b/runner.go
@@ -1,3 +1,6 @@
+func runBuild(ctx context.Context) error {
+	cmd := exec.CommandContext(ctx, "make", "build")
+	return cmd.Output()
+}
`)
	findings := SLP221{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for CommandContext, got %d", len(findings))
	}
}
