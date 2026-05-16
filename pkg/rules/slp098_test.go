package rules

import "testing"

func TestSLP098_FiresOnRouteWithoutTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/routes.go b/routes.go
--- a/routes.go
+++ b/routes.go
@@ -1,1 +1,3 @@
+  mux.HandleFunc("/api/items", itemsHandler)
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
}

func TestSLP098_FiresOnExpressRouteWithoutTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/router.ts b/router.ts
--- a/router.ts
+++ b/router.ts
@@ -1,1 +1,3 @@
+  app.post("/api/users", usersHandler);
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
}

func TestSLP098_NoFireWhenTestAlsoModified(t *testing.T) {
	d := parseDiff(t, `diff --git a/routes.go b/routes.go
--- a/routes.go
+++ b/routes.go
@@ -1,1 +1,3 @@
+  mux.HandleFunc("/api/items", itemsHandler)
diff --git a/routes_test.go b/routes_test.go
--- a/routes_test.go
+++ b/routes_test.go
@@ -1,1 +1,3 @@
+  func TestItemsHandler(t *testing.T) {
`)
	got := SLP098{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when test also modified, got %d", len(got))
	}
}

func TestSLP098_FiresWhenRelatedTestHasNoAddedLines(t *testing.T) {
	d := parseDiff(t, `diff --git a/routes.go b/routes.go
--- a/routes.go
+++ b/routes.go
@@ -1,1 +1,3 @@
+  mux.HandleFunc("/api/items", itemsHandler)
diff --git a/routes_test.go b/routes_test.go
--- a/routes_test.go
+++ b/routes_test.go
@@ -1,1 +1,0 @@
-  func TestOldItemsHandler(t *testing.T) {}
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding when related test has no added lines, got %d", len(got))
	}
}

func TestSLP098_DoesNotTreatPrefixMatchAsRelatedTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/user.go b/user.go
--- a/user.go
+++ b/user.go
@@ -1,1 +1,3 @@
+  router.Handle("/users", usersHandler)
diff --git a/user_profile_test.go b/user_profile_test.go
--- a/user_profile_test.go
+++ b/user_profile_test.go
@@ -1,1 +1,3 @@
+  func TestUserProfile(t *testing.T) {}
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding when only prefixed test file changed, got %d", len(got))
	}
}

func TestSLP098_FiresWhenRelatedTestInTestsDir(t *testing.T) {
	d := parseDiff(t, `diff --git a/routes.go b/routes.go
--- a/routes.go
+++ b/routes.go
@@ -1,1 +1,3 @@
+  mux.HandleFunc("/api/users", usersHandler)
diff --git a/tests/routes/users.test.ts b/tests/routes/users.test.ts
--- a/tests/routes/users.test.ts
+++ b/tests/routes/users.test.ts
@@ -1,1 +1,0 @@
-  describe("users route", () => {})
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding when related test in tests/ dir has no added lines, got %d", len(got))
	}
}

func TestSLP098_Description(t *testing.T) {
	r := SLP098{}
	if r.ID() != "SLP098" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}

func TestSLP098_FastifyRoute(t *testing.T) {
	d := parseDiff(t, `diff --git a/server.ts b/server.ts
--- a/server.ts
+++ b/server.ts
@@ -1,1 +1,3 @@
+  fastify.get("/api/items", itemsHandler);
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for Fastify route, got %d", len(got))
	}
}

func TestSLP098_FastAPIRoute(t *testing.T) {
	d := parseDiff(t, `diff --git a/api.py b/api.py
--- a/api.py
+++ b/api.py
@@ -1,1 +1,3 @@
+  @app.get("/items")
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for FastAPI route, got %d", len(got))
	}
}

func TestSLP098_GinRoute(t *testing.T) {
	d := parseDiff(t, `diff --git a/routes.go b/routes.go
--- a/routes.go
+++ b/routes.go
@@ -1,1 +1,3 @@
+  r.GET("/api/users", usersHandler)
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for Gin route, got %d", len(got))
	}
}

func TestSLP098_NewRouteFileByName(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/api/routes.ts b/src/api/routes.ts
--- /dev/null
+++ b/src/api/routes.ts
@@ -0,0 +1,3 @@
+  export const config = { runtime: "edge" };
+  export default function handler(req: Request) {
+    return new Response("ok");
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for new route file by name, got %d", len(got))
	}
	// export default function handler matches a route pattern, so the message
	// is the route-pattern one (not the naming-convention fallback).
}

func TestSLP098_NewRouteFileByNameNoFireWithTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/api/routes.ts b/src/api/routes.ts
--- /dev/null
+++ b/src/api/routes.ts
@@ -0,0 +1,3 @@
+  export const config = { runtime: "edge" };
+  export default function handler(req: Request) {
+    return new Response("ok");
diff --git a/src/api/routes.test.ts b/src/api/routes.test.ts
--- /dev/null
+++ b/src/api/routes.test.ts
@@ -0,0 +1,3 @@
+  test("routes handler", () => {});
`)
	got := SLP098{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when test file also added, got %d", len(got))
	}
}

func TestSLP098_NewControllersFileByName(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/controllers/userController.ts b/src/controllers/userController.ts
--- /dev/null
+++ b/src/controllers/userController.ts
@@ -0,0 +1,3 @@
+  export function getUser(req: Request) { return null; }
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for new controller file, got %d", len(got))
	}
}

func TestSLP098_DjangoURL(t *testing.T) {
	d := parseDiff(t, `diff --git a/myapp/urls.py b/myapp/urls.py
--- a/myapp/urls.py
+++ b/myapp/urls.py
@@ -1,1 +1,3 @@
+  path("api/items/", views.items_list),
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for Django URL, got %d", len(got))
	}
}

func TestSLP098_tFPCRouter(t *testing.T) {
	d := parseDiff(t, `diff --git a/trpc/router.ts b/trpc/router.ts
--- a/trpc/router.ts
+++ b/trpc/router.ts
@@ -1,1 +1,3 @@
+  publicProcedure.query("getItems", () => [])
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for tRPC router, got %d", len(got))
	}
}

func TestSLP098_NewEndpointsFileByName(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/endpoints.ts b/src/endpoints.ts
--- /dev/null
+++ b/src/endpoints.ts
@@ -0,0 +1,2 @@
+  export const API_BASE = "/api/v2";
+  export function fetchData() {}
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for new endpoints file, got %d", len(got))
	}
}

func TestSLP098_NewRouteFileNamingOnly(t *testing.T) {
	// File named "routes.ts" with no explicit route patterns — detected by name only
	d := parseDiff(t, `diff --git a/src/routes.ts b/src/routes.ts
--- /dev/null
+++ b/src/routes.ts
@@ -0,0 +1,2 @@
+  export const API_VERSION = "v2";
+  export function setup(app: any) {}
`)
	got := SLP098{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for route file by naming convention only, got %d", len(got))
	}
	if got[0].Message != "new route/handler file added without corresponding test file" {
		t.Errorf("expected naming-convention message, got: %s", got[0].Message)
	}
}
