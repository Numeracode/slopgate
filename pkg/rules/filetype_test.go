package rules

import "testing"

func TestNormalizedSlashPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "windows mixed case", in: `FOO\Bar`, want: "foo/bar"},
		{name: "openapi path", in: `Docs\Contracts\OpenAPI\Whimsy-API.OpenAPI.JSON`, want: "docs/contracts/openapi/whimsy-api.openapi.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizedSlashPath(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestGeneratedArtifactPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "root generated dir", in: "generated/client.ts", want: true},
		{name: "exact generated dir", in: "generated", want: true},
		{name: "nested generated dir", in: "packages/sdk/src/generated/operations.ts", want: true},
		{name: "windows generated dir", in: `Packages\SDK\Generated\client.ts`, want: true},
		{name: "similar name", in: "generatedx/client.ts", want: false},
		{name: "different dir", in: "packages/sdk/src/notgenerated/operations.ts", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGeneratedArtifactPath(tc.in); got != tc.want {
				t.Fatalf("got %t want %t for %q", got, tc.want, tc.in)
			}
		})
	}
}

func TestOpenAPIArtifactPath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "suffix json", in: "docs/contracts/openapi/whimsy-api.openapi.json", want: true},
		{name: "suffix yaml", in: "docs/contracts/whimsy-api.openapi.yaml", want: true},
		{name: "suffix yml", in: "docs/contracts/whimsy-api.openapi.yml", want: true},
		{name: "root openapi dir", in: "openapi/spec.yaml", want: true},
		{name: "nested openapi dir", in: "docs/contracts/openapi/spec.yaml", want: true},
		{name: "windows openapi dir", in: `Docs\Contracts\OpenAPI\spec.json`, want: true},
		{name: "unrelated extension", in: "openapi/spec.txt", want: false},
		{name: "plain yaml outside dir", in: "docs/contracts/spec.yaml", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOpenAPIArtifactPath(tc.in); got != tc.want {
				t.Fatalf("got %t want %t for %q", got, tc.want, tc.in)
			}
		})
	}
}
