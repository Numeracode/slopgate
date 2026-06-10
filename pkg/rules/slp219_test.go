package rules

import (
	"strings"
	"testing"
)

func TestSLP219_FlagsSharedFieldInHandler(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,3 +1,7 @@
+func SnapshotHandler(w http.ResponseWriter, r *http.Request) {
+	cfg := s.Config
+	_ = cfg
+}
`)
	findings := SLP219{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestSLP219_NoFlagWithLock(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,3 +1,8 @@
+func SnapshotHandler(w http.ResponseWriter, r *http.Request) {
+	s.mu.RLock()
+	defer s.mu.RUnlock()
+	cfg := s.Config
+}
`)
	findings := SLP219{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when lock is held, got %d", len(findings))
	}
}

func TestSLP219_NoFlagNonHandler(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal.go b/internal.go
--- a/internal.go
+++ b/internal.go
@@ -1,2 +1,4 @@
+func refresh() {
+	cfg := s.Config
+}
`)
	findings := SLP219{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings in non-handler context, got %d", len(findings))
	}
}

func TestSLP219_MessageMentionsRace(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,3 +1,5 @@
+func SnapshotHandler(w http.ResponseWriter, r *http.Request) {
+	_ = s.SnapshotManager
+}
`)
	findings := SLP219{}.Check(d)
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding")
	}
	lower := strings.ToLower(findings[0].Message)
	if !strings.Contains(lower, "race") && !strings.Contains(lower, "lock") {
		t.Errorf("expected message about race/lock, got: %s", findings[0].Message)
	}
}
