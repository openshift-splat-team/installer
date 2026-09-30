package clusterapi

import (
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
