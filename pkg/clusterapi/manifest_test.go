package clusterapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
)

func TestObjectsFromManifest(t *testing.T) {
	t.Run("known kind decodes into its go type", func(t *testing.T) {
		got, err := ObjectsFromManifest("cluster.yaml", []byte(`
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
`))
		require.NoError(t, err)
		require.Len(t, got, 1)
		cluster, ok := got[0].Object.(*clusterv1.Cluster)
		require.True(t, ok, "want a typed Cluster, got %T", got[0].Object)
		assert.Equal(t, "test-cluster", cluster.Name)
	})

	t.Run("unknown kind is carried through unstructured", func(t *testing.T) {
		// A provider this installer was never compiled against. Its CRD is
		// installed into the local control plane from the provider's own
		// components, so the object has to survive load without a Go type.
		got, err := ObjectsFromManifest("infra.yaml", []byte(`
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: SomeCloudCluster
metadata:
  name: test-cluster
spec:
  region: somewhere
`))
		require.NoError(t, err)
		require.Len(t, got, 1)
		u, ok := got[0].Object.(*unstructured.Unstructured)
		require.True(t, ok, "want an unstructured object, got %T", got[0].Object)
		assert.Equal(t, "SomeCloudCluster", u.GetKind())

		// The spec has to survive too: it is what the provider reconciles,
		// and the installer never looks inside it.
		region, found, err := unstructured.NestedString(u.Object, "spec", "region")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "somewhere", region)
	})

	t.Run("every document in a multi-document file is decoded", func(t *testing.T) {
		// Putting the Cluster and the infrastructure object in one file is the
		// natural way to write them. sigs.k8s.io/yaml.Unmarshal keeps only the
		// first document and reports no error, so this is the case that used
		// to lose data silently.
		got, err := ObjectsFromManifest("both.yaml", []byte(`
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: SomeCloudCluster
metadata:
  name: test-infra
`))
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.IsType(t, &clusterv1.Cluster{}, got[0].Object)
		assert.Equal(t, "test-cluster", got[0].Object.GetName())
		assert.IsType(t, &unstructured.Unstructured{}, got[1].Object)
		assert.Equal(t, "test-infra", got[1].Object.GetName())

		// Each document keeps its own bytes, so one file round-trips to one
		// asset file per object.
		assert.NotEqual(t, got[0].Data, got[1].Data)
		assert.Contains(t, string(got[1].Data), "SomeCloudCluster")
	})

	t.Run("empty documents are skipped", func(t *testing.T) {
		got, err := ObjectsFromManifest("padded.yaml", []byte(`
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
---
`))
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("a document with no kind is rejected", func(t *testing.T) {
		_, err := ObjectsFromManifest("nokind.yaml", []byte("metadata:\n  name: x\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nokind.yaml")
		assert.Contains(t, err.Error(), "Kind")
	})

	t.Run("malformed yaml is rejected and names the file", func(t *testing.T) {
		_, err := ObjectsFromManifest("broken.yaml", []byte("\tnot: [valid"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "broken.yaml")
	})

	t.Run("an error names the document it came from", func(t *testing.T) {
		_, err := ObjectsFromManifest("second.yaml", []byte(`
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ok
---
metadata:
  name: missing-kind
`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "document 2")
	})
}

// `destroy cluster`.
func TestDecodeErrorDoesNotEchoTheManifest(t *testing.T) {
	const secret = "super-secret-value-that-must-not-be-logged"
	// A document with no kind, which is what triggers the offending error.
	manifest := []byte("apiVersion: v1\nmetadata:\n  name: test\nstringData:\n  token: " + secret + "\n")

	_, err := ObjectsFromManifest("artifacts/object.yaml", manifest)
	if err == nil {
		t.Fatal("expected an error for a document with no kind")
	}

	msg := err.Error()
	if strings.Contains(msg, secret) {
		t.Errorf("the decode error echoed the manifest contents:\n%s", msg)
	}
	if len(msg) > 300 {
		t.Errorf("the decode error is %d characters, which is long enough to be carrying the document:\n%s",
			len(msg), msg)
	}
	// The location must survive, or the message is useless.
	if !strings.Contains(msg, "artifacts/object.yaml") {
		t.Errorf("the decode error does not name the file it came from: %s", msg)
	}
	if !strings.Contains(msg, "Kind") {
		t.Errorf("the decode error does not say what was wrong: %s", msg)
	}
}

// TestRedactDecodeErrorKeepsDiagnosticDetail checks the redaction is a scalpel:
// a YAML syntax error carries a line number and no content, and must survive
// intact.
func TestRedactDecodeErrorKeepsDiagnosticDetail(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "missing kind loses only the document",
			in:   "Object 'Kind' is missing in '{\"stringData\":{\"token\":\"secret\"}}'",
			want: "Object 'Kind' is missing",
		},
		{
			name: "missing apiVersion loses only the document",
			in:   "Object 'apiVersion' is missing in '{\"kind\":\"Secret\"}'",
			want: "Object 'apiVersion' is missing",
		},
		{
			name: "a syntax error is passed through with its line number",
			in:   "error converting YAML to JSON: yaml: line 5: did not find expected key",
			want: "error converting YAML to JSON: yaml: line 5: did not find expected key",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactDecodeError(errors.New(tc.in)); got != tc.want {
				t.Errorf("redactDecodeError() = %q, want %q", got, tc.want)
			}
		})
	}

	long := redactDecodeError(errors.New(strings.Repeat("x", 5000)))
	if len(long) > maxDecodeErrorLength+len("... (truncated)") {
		t.Errorf("an unanticipated error shape was not bounded: %d characters", len(long))
	}
}

// TestObjectsFromManifestSkipsCommentOnlyDocuments covers the shape every one
// of these files actually has: a comment block above the first `---`.
//
// A comment block is not empty as bytes, so the blank-document guard does not
// catch it, and it resolves to no kind, so the unresolved-kind fallback used
// to accept it as an object. Provision then failed at cl.Create with
// "unstructured object has no kind" -- after the local control plane was up
// and after the earlier manifests in the same file had already been created.
