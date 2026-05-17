package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSLP081(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected int
	}{
		{
			name: "tsx without React import but plain JSX is ok under automatic runtime",
			diff: `diff --git a/src/components/Button.tsx b/src/components/Button.tsx
index 123..456 100644
--- a/src/components/Button.tsx
+++ b/src/components/Button.tsx
@@ -1,5 +1,8 @@
-const Button = () => {
-  return <button>Click me</button>
+const Button = () => {
+  return <button className="btn">Click me</button>
 }
 export default Button
`,
			expected: 0,
		},
		{
			name: "jsx without React import but plain JSX is ok under automatic runtime",
			diff: `diff --git a/src/App.jsx b/src/App.jsx
index 123..456 100644
--- a/src/App.jsx
+++ b/src/App.jsx
@@ -1,5 +1,6 @@
-const App = () => <div>Hello</div>
+const App = () => <div className="app">Hello</div>
 export default App
`,
			expected: 0,
		},
		{
			name: "tsx with React namespace usage and no import still fires",
			diff: `diff --git a/src/components/Button.tsx b/src/components/Button.tsx
index 123..456 100644
--- a/src/components/Button.tsx
+++ b/src/components/Button.tsx
@@ -1,5 +1,8 @@
-const Button = () => {
-  return <>Click me</>
+const Button = () => {
+  return <React.Fragment>Click me</React.Fragment>
 }
 export default Button
`,
			expected: 1,
		},
		{
			name: "tsx with React import in context is ok",
			diff: `diff --git a/src/components/Button.tsx b/src/components/Button.tsx
index 123..456 100644
--- a/src/components/Button.tsx
+++ b/src/components/Button.tsx
@@ -1,5 +1,5 @@
 import React from 'react'
 const Button = () => {
-  return <>Click me</>
+  return <React.Fragment>Click me</React.Fragment>
 }
 export default Button
`,
			expected: 0,
		},
		{
			name: "ts file without JSX is ok",
			diff: `diff --git a/src/utils.ts b/src/utils.ts
index 123..456 100644
--- a/src/utils.ts
+++ b/src/utils.ts
@@ -1,3 +1,5 @@
-const add = (a: number, b: number) => a + b
+const add = (a: number, b: number) => a + b
 export { add }
`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.diff)
			r := SLP081{}
			findings := r.Check(d)

			if len(findings) != tt.expected {
				t.Errorf("expected %d findings, got %d", tt.expected, len(findings))
				for _, f := range findings {
					t.Logf("  - %s:%d: %s", f.File, f.Line, f.Message)
				}
			}
		})
	}
}

func TestSLP081_ReactNamespacePattern(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"const Button = () => <button>Click</button>", false},
		{"export default function App() { return <React.Fragment /> }", true},
		{"const node = React.createElement('div')", true},
		{"const x = 5", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			d := parseDiff(t, `diff --git a/test.tsx b/test.tsx
index 123..456 100644
--- a/test.tsx
+++ b/test.tsx
@@ -1,1 +1,1 @@
-`+tt.line+`
+`+tt.line+`
`)
			r := SLP081{}
			findings := r.Check(d)

			hasFinding := len(findings) > 0
			if hasFinding != tt.expected {
				t.Errorf("line %q: expected React namespace finding=%v, got %v", tt.line, tt.expected, hasFinding)
			}
		})
	}
}

func TestSLP081_FallsBackToSnapshotFileImportsOutsideHunk(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "src/components")
	if err := os.MkdirAll(filePath, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `import React from "react";

export function Button() {
  return <React.Fragment>Click me</React.Fragment>;
}
`
	target := filepath.Join(filePath, "Button.tsx")
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	d := parseDiffWithRoot(t, root, `diff --git a/src/components/Button.tsx b/src/components/Button.tsx
--- a/src/components/Button.tsx
+++ b/src/components/Button.tsx
@@ -2,2 +2,5 @@
 export function Button() {
-  return <React.Fragment>Click me</React.Fragment>;
+  const className = "btn";
+  return <React.Fragment>{className}</React.Fragment>;
+}
`)

	got := SLP081{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings with snapshot file React import, got %d: %+v", len(got), got)
	}
}

func TestSLP081_PseudoImportTextDoesNotSuppressFinding(t *testing.T) {
	tests := []struct {
		name string
		diff string
	}{
		{
			name: "comment pseudo import",
			diff: `diff --git a/test.tsx b/test.tsx
--- a/test.tsx
+++ b/test.tsx
@@ -1,2 +1,3 @@
 // import React from 'react'
+const node = <React.Fragment />
 const footer = true
`,
		},
		{
			name: "string literal pseudo import",
			diff: `diff --git a/test.tsx b/test.tsx
--- a/test.tsx
+++ b/test.tsx
@@ -1,2 +1,3 @@
 const note = "import React from 'react'"
+const node = <React.Fragment />
 const footer = true
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.diff)
			got := SLP081{}.Check(d)
			if len(got) != 1 {
				t.Fatalf("expected 1 finding when only pseudo-import text is present, got %d: %+v", len(got), got)
			}
		})
	}
}
