package rules

import (
	"testing"
)

func TestSLP085(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected int
	}{
		{
			name: "SQL string concatenation",
			diff: `diff --git a/src/services/userService.js b/src/services/userService.js
index 123..456 100644
--- a/src/services/userService.js
+++ b/src/services/userService.js
@@ -1,5 +1,8 @@
-const getUser = (id) => {
-  const query = "SELECT * FROM users WHERE id = " + id
+const getUser = (id) => {
+  const query = "SELECT * FROM users WHERE id = " + id
   return db.query(query)
 }
`,
			expected: 1,
		},
		{
			name: "SQL template literal",
			diff: "diff --git a/src/services/userService.js b/src/services/userService.js\n" +
				"index 123..456 100644\n" +
				"--- a/src/services/userService.js\n" +
				"+++ b/src/services/userService.js\n" +
				"@@ -1,5 +1,8 @@\n" +
				"-const getUser = (id) => {\n" +
				"-  const query = \"SELECT * FROM users WHERE id = \" + id\n" +
				"+const getUser = (id) => {\n" +
				"+  const query = `SELECT * FROM users WHERE id = ${id}`\n" +
				"   return db.query(query)\n" +
				" }\n",
			expected: 1,
		},
		{
			name: "parameterized query is ok",
			diff: `diff --git a/src/services/userService.js b/src/services/userService.js
index 123..456 100644
--- a/src/services/userService.js
+++ b/src/services/userService.js
@@ -1,5 +1,8 @@
-const getUser = (id) => {
-  const query = "SELECT * FROM users WHERE id = ?"
-  return db.query(query, [id])
+const getUser = (id) => {
+  const query = "SELECT * FROM users WHERE id = ?"
+  return db.query(query, [id])
 }
`,
			expected: 0,
		},
		{
			name: "SQL in Go with string concat",
			diff: `diff --git a/internal/db/user.go b/internal/db/user.go
index 123..456 100644
--- a/internal/db/user.go
+++ b/internal/db/user.go
@@ -1,5 +1,8 @@
-func GetUser(id int) error {
-  query := "SELECT * FROM users WHERE id = " + strconv.Itoa(id)
+func GetUser(id int) error {
+  query := "SELECT * FROM users WHERE id = " + strconv.Itoa(id)
   return db.QueryRow(query).Scan(&user)
 }
`,
			expected: 1,
		},
		{
			name: "SQLINSERT with concat",
			diff: `diff --git a/src/services/userService.js b/src/services/userService.js
index 123..456 100644
--- a/src/services/userService.js
+++ b/src/services/userService.js
@@ -1,5 +1,8 @@
-const createUser = (name, email) => {
-  const query = "INSERT INTO users (name, email) VALUES ('" + name + "', '" + email + "')"
+const createUser = (name, email) => {
+  const query = "INSERT INTO users (name, email) VALUES ('" + name + "', '" + email + "')"
   return db.query(query)
 }
`,
			expected: 1,
		},
		{
			name: "regex literal with SQL-like keyword and quantifier is ok",
			diff: `diff --git a/pkg/rules/routes.go b/pkg/rules/routes.go
index 123..456 100644
--- a/pkg/rules/routes.go
+++ b/pkg/rules/routes.go
@@ -1,2 +1,3 @@
 package rules
+
+var routeRe = regexp.MustCompile(` + "`" + `(?i)(?:get|post|delete|patch)\s+["']/` + "`" + `)
`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.diff)
			r := SLP085{}
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
