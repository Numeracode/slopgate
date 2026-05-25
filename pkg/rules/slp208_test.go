package rules

import "testing"

func TestSLP208(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFire bool
	}{
		{
			name: "default before required — function declaration",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+function createUser(name = "guest", role) {
+  return { name, role }
+}
`,
			wantFire: true,
		},
		{
			name: "default before required — arrow function",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+const createUser = (name = "guest", role) => {
+  return { name, role }
+}
`,
			wantFire: true,
		},
		{
			name: "default in middle — three params",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+function foo(a, b = 1, c) {
+  return a + b + c
+}
`,
			wantFire: true,
		},
		{
			name: "defaults last — no fire",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+function createUser(name, role = "viewer") {
+  return { name, role }
+}
`,
			wantFire: false,
		},
		{
			name: "all defaults — no fire",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+function createUser(name = "guest", role = "viewer") {
+  return { name, role }
+}
`,
			wantFire: false,
		},
		{
			name: "single param with default — no fire",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,2 @@
+function createUser(name = "guest") {
+}
`,
			wantFire: false,
		},
		{
			name: "no defaults at all — no fire",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,2 @@
+function createUser(name, role) {
+}
`,
			wantFire: false,
		},
		{
			name: "comment line — no fire",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,2 @@
+// function foo(a = 1, b) { }
+const x = 1
`,
			wantFire: false,
		},
		{
			name: "Go file — no fire (wrong language)",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -0,0 +1,3 @@
+func createUser(name string, role string) error {
+  return nil
+}
`,
			wantFire: false,
		},
		{
			name: "async arrow with default before required",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,3 @@
+const fetchUser = async (id = 0, token) => {
+  return await api.get(id, token)
+}
`,
			wantFire: true,
		},
	}

	r := SLP208{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.input)
			findings := r.Check(d)
			if tt.wantFire && len(findings) == 0 {
				t.Errorf("expected finding but got none")
			}
			if !tt.wantFire && len(findings) > 0 {
				t.Errorf("expected no finding but got %d: %v", len(findings), findings)
			}
		})
	}
}
