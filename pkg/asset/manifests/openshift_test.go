package manifests

import (
	"testing"
)

// TestCountManifestDocuments covers the shapes that decide whether a
// user-supplied extra manifest is accepted. The two-object case is the one
// that matters: before this check existed, the bootstrap node applied the
// first object and dropped the second silently.
func TestCountManifestDocuments(t *testing.T) {
	cases := []struct {
		name string
		data string
		want int
	}{{
		name: "a single object",
		data: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n",
		want: 1,
	}, {
		name: "a licence header and a leading separator is still one object",
		data: "# Copyright\n# notice\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n",
		want: 1,
	}, {
		name: "two objects, the shape that loses one",
		data: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n",
		want: 2,
	}, {
		name: "a trailing separator adds no object",
		data: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\n",
		want: 1,
	}, {
		name: "comments only",
		data: "# nothing here\n",
		want: 0,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := countManifestDocuments([]byte(tc.data))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d documents, want %d", got, tc.want)
			}
		})
	}
}
