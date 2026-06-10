package rules

import (
	"strings"
	"testing"
)

func TestSLP216_FlagsShallowErrorLogging(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/transfers/upload.js b/api/transfers/upload.js
--- a/api/transfers/upload.js
+++ b/api/transfers/upload.js
@@ -10,6 +10,7 @@ async function upload() {
   try {
     await doUpload()
   } catch (err) {
+    logger.error('upload failed:', err.message)
   }
 }
`)
	findings := SLP216{}.Check(d)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "err.message") {
		t.Errorf("expected message about err.message, got: %s", findings[0].Message)
	}
}

func TestSLP216_NoWarningWhenFullErrorPassed(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/transfers/upload.js b/api/transfers/upload.js
--- a/api/transfers/upload.js
+++ b/api/transfers/upload.js
@@ -10,6 +10,7 @@ async function upload() {
   try {
     await doUpload()
   } catch (err) {
+    logger.error('upload failed:', err)
   }
 }
`)
	findings := SLP216{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when full error is passed, got %d", len(findings))
	}
}

func TestSLP216_NoWarningWhenErrorInterpolated(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/transfers/upload.js b/api/transfers/upload.js
--- a/api/transfers/upload.js
+++ b/api/transfers/upload.js
@@ -10,6 +10,7 @@ async function upload() {
   try {
     await doUpload()
   } catch (err) {
+    logger.error(`+"`"+`upload failed: ${err}`+"`"+`)
   }
 }
`)
	findings := SLP216{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when error is interpolated, got %d", len(findings))
	}
}

func TestSLP216_IgnoresNonLogStatements(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/transfers/upload.js b/api/transfers/upload.js
--- a/api/transfers/upload.js
+++ b/api/transfers/upload.js
@@ -10,6 +10,7 @@ async function upload() {
   try {
     await doUpload()
   } catch (err) {
+    const msg = err.message
   }
 }
`)
	findings := SLP216{}.Check(d)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-log statement, got %d", len(findings))
	}
}
