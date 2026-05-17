package rules

import "testing"

func TestNormalizedSlashPath(t *testing.T) {
	if got := normalizedSlashPath(""); got != "" {
		t.Fatalf("expected empty path to stay empty, got %q", got)
	}
	if got := normalizedSlashPath(`Docs\Contracts\OpenAPI\Whimsy-API.OpenAPI.JSON`); got != "docs/contracts/openapi/whimsy-api.openapi.json" {
		t.Fatalf("unexpected normalized path: %q", got)
	}
}

func TestGeneratedArtifactPath(t *testing.T) {
	if isGeneratedArtifactPath("") {
		t.Fatal("empty path should not be treated as generated")
	}
	if !isGeneratedArtifactPath("packages/sdk/src/generated/operations.ts") {
		t.Fatal("generated SDK path should be detected")
	}
	if isGeneratedArtifactPath("packages/sdk/src/operations.ts") {
		t.Fatal("non-generated SDK path should not be detected")
	}
}

func TestOpenAPIArtifactPath(t *testing.T) {
	if isOpenAPIArtifactPath("") {
		t.Fatal("empty path should not be treated as OpenAPI")
	}
	if !isOpenAPIArtifactPath("docs/contracts/openapi/whimsy-api.openapi.json") {
		t.Fatal("OpenAPI filename suffix should be detected")
	}
	if !isOpenAPIArtifactPath("docs/contracts/openapi/spec.yaml") {
		t.Fatal("OpenAPI directory should be detected for YAML specs")
	}
	if isOpenAPIArtifactPath("docs/contracts/spec.yaml") {
		t.Fatal("plain YAML outside OpenAPI paths should not be detected")
	}
}
