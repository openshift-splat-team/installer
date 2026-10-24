package clusterapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/manifests/capiutils"
	"github.com/openshift/installer/pkg/clusterapi"
)

// decodeFixture builds RuntimeFiles the way Load does -- through the manifest
// decoder -- rather than by constructing objects in Go. A hand-built fixture
// would carry a populated TypeMeta that the real path never produces, and that
// difference is the whole subject of these tests.
func decodeFixture(t *testing.T, doc string) []*asset.RuntimeFile {
	t.Helper()
	decoded, err := clusterapi.ObjectsFromManifest("fixture.yaml", []byte(doc))
	require.NoError(t, err)
	files := make([]*asset.RuntimeFile, 0, len(decoded))
	for _, d := range decoded {
		files = append(files, &asset.RuntimeFile{
			File:   asset.File{Filename: "fixture.yaml", Data: d.Data},
			Object: d.Object,
		})
	}
	return files
}

func TestHasNamespace(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want bool
	}{{
		name: "generated namespace read back from disk",
		// This is the case that regressed: the namespace Generate writes, read
		// back by Load on a second command. Scheme.Convert leaves TypeMeta
		// empty, so a TypeMeta-based check reports false here and a duplicate
		// namespace is appended.
		doc:  "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + capiutils.Namespace + "\n",
		want: true,
	}, {
		name: "user-supplied namespace in a multi-document file",
		doc: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + capiutils.Namespace + "\n" +
			"---\napiVersion: cluster.x-k8s.io/v1beta1\nkind: Cluster\nmetadata:\n  name: example\n",
		want: true,
	}, {
		name: "some other namespace",
		doc:  "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: openshift-config\n",
		want: false,
	}, {
		name: "no namespace at all",
		doc:  "apiVersion: cluster.x-k8s.io/v1beta1\nkind: Cluster\nmetadata:\n  name: example\n",
		want: false,
	}, {
		name: "unresolved partner kind that merely shares the name",
		// Stays unstructured because the installer has no type for it. It is
		// named like the namespace but is not one, so it must not satisfy the
		// check and leave the real namespace unwritten.
		doc: "apiVersion: example.partner.io/v1\nkind: Widget\nmetadata:\n  name: " +
			capiutils.Namespace + "\n",
		want: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasNamespace(decodeFixture(t, tc.doc)))
		})
	}
}

// TestNamespaceRuntimeFileRoundTrips closes the loop the two functions form:
// what namespaceRuntimeFile emits must be recognised by hasNamespace after it
// has been through the decoder, which is what happens on the next command.
func TestNamespaceRuntimeFileRoundTrips(t *testing.T) {
	nsFile, err := namespaceRuntimeFile()
	require.NoError(t, err)
	assert.True(t, hasNamespace([]*asset.RuntimeFile{nsFile}), "not recognised as written")
	assert.True(t, hasNamespace(decodeFixture(t, string(nsFile.Data))), "not recognised after a disk round trip")
}
