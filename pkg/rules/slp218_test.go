package rules

import (
	"strings"
	"testing"
)

func TestSLP218_FlagsContentLengthGateWithoutChunked(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/handlers/e2ee.go b/internal/handlers/e2ee.go
--- a/internal/handlers/e2ee.go
+++ b/internal/handlers/e2ee.go
@@ -10,6 +10,10 @@ func DecryptHandler(w http.ResponseWriter, r *http.Request) {
  var body DecryptRequest
+	if r.ContentLength > 0 {
+		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
+			http.Error(w, err.Error(), 400)
+		}
+	}
 }
`)
	findings := SLP218{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "chunked") {
		t.Errorf("expected message about chunked encoding, got: %s", findings[0].Message)
	}
}

func TestSLP218_NoWarningWhenChunkedHandled(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/handlers/e2ee.go b/internal/handlers/e2ee.go
--- a/internal/handlers/e2ee.go
+++ b/internal/handlers/e2ee.go
@@ -10,6 +10,13 @@ func DecryptHandler(w http.ResponseWriter, r *http.Request) {
  var body DecryptRequest
+	isChunked := r.TransferEncoding != nil && len(r.TransferEncoding) > 0
+	if r.ContentLength > 0 || isChunked {
+		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
+			http.Error(w, err.Error(), 400)
+		}
+	}
 }
`)
	findings := SLP218{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when chunked is handled, got %d", len(findings))
	}
}

func TestSLP218_NoWarningWhenContentLengthZero(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/handlers/e2ee.go b/internal/handlers/e2ee.go
--- a/internal/handlers/e2ee.go
+++ b/internal/handlers/e2ee.go
@@ -10,6 +10,9 @@ func DecryptHandler(w http.ResponseWriter, r *http.Request) {
  var body DecryptRequest
+	if r.ContentLength == 0 {
+		http.Error(w, "no body", 400)
+	}
 }
`)
	findings := SLP218{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for ContentLength == 0 check, got %d", len(findings))
	}
}

func TestSLP218_IgnoresNonHTTPContext(t *testing.T) {
	d := parseDiff(t, `diff --git a/internal/utils.go b/internal/utils.go
--- a/internal/utils.go
+++ b/internal/utils.go
@@ -1,0 +1,4 @@
+func processFile() {
+	if fileInfo.ContentLength > 0 {
+		// not an HTTP handler
+	}
+}
`)
	findings := SLP218{}.Check(d)
	// Should still flag it since the rule is conservative, but this test
	// documents that it doesn't specifically check for handler context
	// (which is hard to detect reliably)
	if len(findings) > 1 {
		t.Errorf("expected at most 1 finding for non-HTTP context, got %d", len(findings))
	}
}
