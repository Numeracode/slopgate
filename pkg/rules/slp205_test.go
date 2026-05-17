package rules

import "testing"

func TestSLP205_OpenAPIPathMergeOrder(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want int
	}{
		{
			name: "flags spec paths before oas path maps",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -10,6 +10,11 @@
 function buildSpec(spec) {
+  spec.paths = {
+    ...(spec.paths || {}),
+    ...oas6Paths,
+    ...oas7Paths,
+  };
   return spec;
 }
`,
			want: 1,
		},
		{
			name: "flags Whimsy 606 bad merge order regression",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -1813,7 +2244,7 @@ function buildSpec() {
       path.join(apisRoot, 'app.js'),
     ],
   });
-  spec.paths = { ...oas6Paths, ...(spec.paths || {}) };
+  spec.paths = { ...(spec.paths || {}), ...oas6Paths, ...oas7Paths };
   return withGeneratedOperationIds(spec);
 }
`,
			want: 1,
		},
		{
			name: "flags generated spread added to existing bad block",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -10,8 +10,9 @@
 function buildSpec(spec) {
   spec.paths = {
     ...(spec.paths || {}),
+    ...oas7Paths,
   };
 }
`,
			want: 1,
		},
		{
			name: "does not flag corrected order",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -10,6 +10,11 @@
 function buildSpec(spec) {
+  spec.paths = {
+    ...oas6Paths,
+    ...oas7Paths,
+    ...(spec.paths || {}),
+  };
   return spec;
 }
`,
			want: 0,
		},
		{
			name: "does not flag unrelated schema merge",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -10,3 +10,8 @@
 function buildSpec(spec) {
+  spec.components.schemas = {
+    ...(spec.components?.schemas || {}),
+    ...oas7Schemas,
+  };
 }
`,
			want: 0,
		},
		{
			name: "does not flag tests",
			diff: `diff --git a/api/tests/openapi_spec.test.js b/api/tests/openapi_spec.test.js
--- a/api/tests/openapi_spec.test.js
+++ b/api/tests/openapi_spec.test.js
@@ -1,3 +1,9 @@
 test('fixture', () => {
+  spec.paths = {
+    ...(spec.paths || {}),
+    ...oas7Paths,
+  };
 });
`,
			want: 0,
		},
		{
			name: "does not flag context only bad order",
			diff: `diff --git a/api/src/lib/openapi.js b/api/src/lib/openapi.js
--- a/api/src/lib/openapi.js
+++ b/api/src/lib/openapi.js
@@ -10,8 +10,9 @@
 function buildSpec(spec) {
   spec.paths = {
     ...(spec.paths || {}),
     ...oas7Paths,
   };
+  spec.info.title = 'Whimsy';
 }
`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SLP205{}.Check(parseDiff(t, tt.diff))
			if len(got) != tt.want {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.want, got)
			}
		})
	}
}

func TestSLP205_IDAndDescription(t *testing.T) {
	var r SLP205
	if r.ID() != "SLP205" {
		t.Fatalf("ID() = %q, want SLP205", r.ID())
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Fatalf("DefaultSeverity() = %s, want warn", r.DefaultSeverity())
	}
	if r.Description() == "" {
		t.Fatalf("Description() is empty")
	}
}
