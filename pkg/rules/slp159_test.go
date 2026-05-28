package rules

import (
	"strings"
	"testing"
)

func TestSLP159_FiresOnSpawnSyncWithoutTimeoutInTestFile(t *testing.T) {
	// The CR-caught precedent on whimsy #1156: connector_sdk_generation.test.js
	// defined a runNode helper using spawnSync with no `timeout:` option.
	d := parseDiff(t, `diff --git a/api/tests/connector_sdk_generation.test.js b/api/tests/connector_sdk_generation.test.js
--- a/api/tests/connector_sdk_generation.test.js
+++ b/api/tests/connector_sdk_generation.test.js
@@ -1,3 +1,6 @@
 'use strict';
+function runNode(args) {
+  return spawnSync(process.execPath, args, { cwd: repoRoot, encoding: 'utf8' });
+}
`)
	got := SLP159{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding on spawnSync without timeout, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "timeout") {
		t.Errorf("message should mention timeout: %q", got[0].Message)
	}
}

func TestSLP159_IgnoresSpawnSyncWithSameLineTimeout(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/tests/x.test.js b/api/tests/x.test.js
--- a/api/tests/x.test.js
+++ b/api/tests/x.test.js
@@ -1,3 +1,5 @@
 'use strict';
+function runNode(args) {
+  return spawnSync(process.execPath, args, { cwd: repoRoot, encoding: 'utf8', timeout: 30000 });
+}
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when timeout present same-line, got %d: %+v", len(got), got)
	}
}

func TestSLP159_IgnoresSpawnSyncWithMultilineTimeout(t *testing.T) {
	// Multi-line options-object — the more common formatter output.
	d := parseDiff(t, `diff --git a/api/tests/x.test.js b/api/tests/x.test.js
--- a/api/tests/x.test.js
+++ b/api/tests/x.test.js
@@ -1,3 +1,9 @@
 'use strict';
+function runNode(args) {
+  return spawnSync(process.execPath, args, {
+    cwd: repoRoot,
+    encoding: 'utf8',
+    timeout: 30000,
+  });
+}
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when timeout in multi-line opts, got %d: %+v", len(got), got)
	}
}

func TestSLP159_IgnoresSpawnSyncWithSpreadOpts(t *testing.T) {
	// `...opts` spread could carry a timeout; conservatively suppress.
	d := parseDiff(t, `diff --git a/api/tests/x.test.js b/api/tests/x.test.js
--- a/api/tests/x.test.js
+++ b/api/tests/x.test.js
@@ -1,3 +1,5 @@
 'use strict';
+function runNode(args, opts) {
+  return spawnSync(process.execPath, args, { cwd: repoRoot, ...opts });
+}
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when ...opts spread is present, got %d: %+v", len(got), got)
	}
}

func TestSLP159_IgnoresSpawnInNonTestFile(t *testing.T) {
	// Production scripts/services often run subprocesses without a timeout
	// (servers, watchers). The rule is test-scoped — out-of-scope here.
	d := parseDiff(t, `diff --git a/scripts/build.js b/scripts/build.js
--- a/scripts/build.js
+++ b/scripts/build.js
@@ -1,2 +1,3 @@
 'use strict';
+spawnSync('node', ['build.mjs'], { stdio: 'inherit' });
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings in non-test file, got %d: %+v", len(got), got)
	}
}

func TestSLP159_FiresOnExecSyncWithoutTimeout(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/tests/a.spec.js b/api/tests/a.spec.js
--- a/api/tests/a.spec.js
+++ b/api/tests/a.spec.js
@@ -1,2 +1,3 @@
 'use strict';
+const out = execSync('git rev-parse HEAD', { encoding: 'utf8' });
`)
	got := SLP159{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding on execSync without timeout, got %d: %+v", len(got), got)
	}
}

func TestSLP159_FiresInTypeScriptTestUnderTestsDir(t *testing.T) {
	// `__tests__/` is a Jest convention — files there don't need .test.* suffix.
	d := parseDiff(t, `diff --git a/src/__tests__/runner.ts b/src/__tests__/runner.ts
--- a/src/__tests__/runner.ts
+++ b/src/__tests__/runner.ts
@@ -1,2 +1,3 @@
 import { spawn } from 'child_process';
+const child = spawn('node', ['runner.js'], { stdio: 'inherit' });
`)
	got := SLP159{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for spawn in __tests__/runner.ts, got %d: %+v", len(got), got)
	}
}

func TestSLP159_FiresOnPythonSubprocessRunWithoutTimeout(t *testing.T) {
	d := parseDiff(t, `diff --git a/tests/test_runner.py b/tests/test_runner.py
--- a/tests/test_runner.py
+++ b/tests/test_runner.py
@@ -1,3 +1,4 @@
 import subprocess
 def test_x():
+    result = subprocess.run(['python', 'script.py'], capture_output=True)
`)
	got := SLP159{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding on subprocess.run without timeout, got %d: %+v", len(got), got)
	}
}

func TestSLP159_IgnoresPythonSubprocessRunWithTimeout(t *testing.T) {
	d := parseDiff(t, `diff --git a/tests/test_runner.py b/tests/test_runner.py
--- a/tests/test_runner.py
+++ b/tests/test_runner.py
@@ -1,3 +1,4 @@
 import subprocess
 def test_x():
+    result = subprocess.run(['python', 'script.py'], timeout=30, capture_output=True)
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings with timeout= keyword, got %d: %+v", len(got), got)
	}
}

func TestSLP159_IgnoresIdentifiersContainingCallNames(t *testing.T) {
	// A user-defined `mySpawn(` should not match the bare `spawn(` regex.
	d := parseDiff(t, `diff --git a/api/tests/x.test.js b/api/tests/x.test.js
--- a/api/tests/x.test.js
+++ b/api/tests/x.test.js
@@ -1,3 +1,4 @@
 'use strict';
+const child = mySpawn(cmd, args);
`)
	got := SLP159{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on user-defined mySpawn, got %d: %+v", len(got), got)
	}
}

func TestSLP159_Description(t *testing.T) {
	r := SLP159{}
	if r.ID() != "SLP159" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
