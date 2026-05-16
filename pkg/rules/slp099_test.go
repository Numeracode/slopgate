package rules

import "testing"

func TestSLP099_FiresOnResponseFieldWithoutTestUpdate(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.go b/response.go
--- a/response.go
+++ b/response.go
@@ -1,5 +1,7 @@
 type ItemResponse struct {
     ID   int    `+"`json:\"id\"`"+`
     Name string `+"`json:\"name\"`"+`
+    Slug string `+"`json:\"slug\"`"+`
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for response field without test")
	}
}

func TestSLP099_NoFireWhenTestAlsoModified(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.go b/response.go
--- a/response.go
+++ b/response.go
@@ -1,3 +1,4 @@
 type ItemResponse struct {
     ID   int    `+"`json:\"id\"`"+`
+    Slug string `+"`json:\"slug\"`"+`
 }
diff --git a/response_test.go b/response_test.go
--- a/response_test.go
+++ b/response_test.go
@@ -1,1 +1,3 @@
+  func TestItemResponse(t *testing.T) {
 `)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when test also modified, got %d", len(got))
	}
}

func TestSLP099_FiresOnUntaggedResponseFieldWithoutTestUpdate(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.go b/response.go
--- a/response.go
+++ b/response.go
@@ -1,3 +1,4 @@
 type ItemResponse struct {
+    Slug string
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for untagged response field without test")
	}
}

func TestSLP099_FiresOnTSInterfacePropWithoutTestUpdate(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.ts b/response.ts
--- a/response.ts
+++ b/response.ts
@@ -1,3 +1,4 @@
 export interface ApiResponse {
+  slug: string;
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for TS response field without test")
	}
}

func TestSLP099_NoFireOnTSInterfacePropWithSpecUpdate(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.ts b/response.ts
--- a/response.ts
+++ b/response.ts
@@ -1,3 +1,4 @@
 export interface ApiResponse {
+  slug: string;
 }
diff --git a/response.spec.ts b/response.spec.ts
--- a/response.spec.ts
+++ b/response.spec.ts
@@ -1,1 +1,3 @@
+  it("covers response field", () => {})
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for TS response field with spec update, got %d", len(got))
	}
}

func TestSLP099_DoesNotTreatContainsMatchAsRelatedTest(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.go b/response.go
--- a/response.go
+++ b/response.go
@@ -1,3 +1,4 @@
 type ItemResponse struct {
+    Slug string `+"`json:\"slug\"`"+`
 }
diff --git a/response_profile_test.go b/response_profile_test.go
--- a/response_profile_test.go
+++ b/response_profile_test.go
@@ -1,1 +1,3 @@
+  func TestResponseProfile(t *testing.T) {}
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding when only containing-stem test file changed")
	}
}

func TestSLP099_IgnoresResultCacheFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/result_cache.ts b/result_cache.ts
--- a/result_cache.ts
+++ b/result_cache.ts
@@ -1,3 +1,4 @@
 export interface ResultCacheEntry {
+  slug: string;
 }
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for result_cache.ts, got %d", len(got))
	}
}

func TestSLP099_IgnoresNonResponseFiles(t *testing.T) {
	d := parseDiff(t, `diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1,1 +1,3 @@
+    Helper string `+"`json:\"helper\"`"+`
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-response file, got %d", len(got))
	}
}

func TestSLP099_Description(t *testing.T) {
	r := SLP099{}
	if r.ID() != "SLP099" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}

func TestSLP099_PythonDataclassField(t *testing.T) {
	d := parseDiff(t, `diff --git a/response.py b/response.py
--- a/response.py
+++ b/response.py
@@ -1,3 +1,4 @@
 @dataclass
 class ItemResponse:
     id: int
+    slug: str
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for Python response field without test")
	}
}

func TestSLP099_PydanticModelField(t *testing.T) {
	d := parseDiff(t, `diff --git a/dto.py b/dto.py
--- a/dto.py
+++ b/dto.py
@@ -1,3 +1,4 @@
 class UserDto(BaseModel):
     name: str
+    email: str
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for Pydantic model field without test")
	}
}

func TestSLP099_BodyKeyword(t *testing.T) {
	d := parseDiff(t, `diff --git a/body.go b/body.go
--- a/body.go
+++ b/body.go
@@ -1,3 +1,4 @@
 type ResponseBody struct {
+    Data string `+"`json:\"data\"`"+`
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for body keyword file without test")
	}
}

func TestSLP099_ReplyKeyword(t *testing.T) {
	d := parseDiff(t, `diff --git a/reply.ts b/reply.ts
--- a/reply.ts
+++ b/reply.ts
@@ -1,3 +1,4 @@
 export interface Reply {
+  text: string;
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for reply keyword file without test")
	}
}

func TestSLP099_ParallelTestDir(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/response.ts b/src/response.ts
--- a/src/response.ts
+++ b/src/response.ts
@@ -1,3 +1,4 @@
 export interface ApiResponse {
+  slug: string;
 }
diff --git a/tests/response.test.ts b/tests/response.test.ts
--- a/tests/response.test.ts
+++ b/tests/response.test.ts
@@ -1,1 +1,2 @@
+  test("response", () => {})
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when parallel test dir modified, got %d", len(got))
	}
}

func TestSLP099_IgnoresResponseUtilFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/response_util.ts b/response_util.ts
--- a/response_util.ts
+++ b/response_util.ts
@@ -1,3 +1,4 @@
 export interface ResponseUtilEntry {
+  slug: string;
 }
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for response_util.ts, got %d", len(got))
	}
}

func TestSLP099_IgnoresResponseFactoryFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/response_factory.go b/response_factory.go
--- a/response_factory.go
+++ b/response_factory.go
@@ -1,3 +1,4 @@
 type ResponseFactory struct {
+    Slug string `+"`json:\"slug\"`"+`
 }
`)
	got := SLP099{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for response_factory.go, got %d", len(got))
	}
}

func TestSLP099_FiresOnResponseTypesFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/response_types.ts b/response_types.ts
--- a/response_types.ts
+++ b/response_types.ts
@@ -1,3 +1,4 @@
 export interface ApiResponse {
+  slug: string;
 }
`)
	got := SLP099{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for response_types.ts (types is ignored, response is keyword)")
	}
}
