package rules

import "testing"

func TestSLP209(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFire bool
	}{
		{
			name: "async arrow with return on some paths but not end",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,6 @@
+const getUser = async (id) => {
+  const user = await db.find(id)
+  if (!user) return null
+  user
+}
+
`,
			wantFire: true,
		},
		{
			name: "async arrow with try/catch missing return",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,8 @@
+const getUser = async (id) => {
+  try {
+    return await db.find(id)
+  } catch (e) {
+    log(e)
+  }
+}
+
`,
			wantFire: true,
		},
		{
			name: "async arrow with return at end — no fire",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,4 @@
+const getUser = async (id) => {
+  const user = await db.find(id)
+  return user
+}
`,
			wantFire: false,
		},
		{
			name: "async arrow no return at all — no fire (side-effect handler)",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,4 @@
+const handler = async (req, res) => {
+  const data = await fetch(req.url)
+  res.json(data)
+}
`,
			wantFire: false,
		},
		{
			name: "async arrow returns null at end — no fire",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,5 @@
+const getUser = async (id) => {
+  const user = await db.find(id)
+  if (!user) return null
+  return user
+}
`,
			wantFire: false,
		},
		{
			name: "sync arrow — no fire (not async)",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,4 @@
+const getUser = (id) => {
+  if (!id) return null
+  db.find(id)
+}
`,
			wantFire: false,
		},
		{
			name: "async function declaration — no fire (not arrow)",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,4 @@
+async function getUser(id) {
+  if (!id) return null
+  db.find(id)
+}
`,
			wantFire: false,
		},
		{
			name: "async arrow with throw at end — no fire",
			input: `diff --git a/src/api.js b/src/api.js
--- a/src/api.js
+++ b/src/api.js
@@ -0,0 +1,5 @@
+const getUser = async (id) => {
+  const user = await db.find(id)
+  if (user) return user
+  throw new Error("not found")
+}
`,
			wantFire: false,
		},
		{
			name: "TypeScript file — fires",
			input: `diff --git a/src/api.ts b/src/api.ts
--- a/src/api.ts
+++ b/src/api.ts
@@ -0,0 +1,5 @@
+const getUser = async (id: string): Promise<User | null> => {
+  const user = await db.find(id)
+  if (!user) return null
+  user
+}
`,
			wantFire: true,
		},
	}

	r := SLP209{}
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
