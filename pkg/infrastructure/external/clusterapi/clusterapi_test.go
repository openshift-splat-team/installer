package clusterapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
	"sigs.k8s.io/controller-runtime/pkg/client"

	externaltypes "github.com/openshift/installer/pkg/types/external"
)

func infraObject() client.Object {
	return infraObjectAtVersion("v1beta2")
}

// infraObjectAtVersion returns a provider infrastructure object at an
// arbitrary version. Unlike the core objects, its version is the provider's
// business and the installer does not constrain it.
func infraObjectAtVersion(version string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("infrastructure.cluster.x-k8s.io/" + version)
	u.SetKind("SomeCloudCluster")
	u.SetName("test-cluster")
	return u
}

// typedCluster returns a Cluster as the asset loader produces it, with its GVK
// set -- ValidateManifests matches on the GVK, not on the Go type.
func typedCluster() client.Object {
	c := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}}
	c.SetGroupVersionKind(clusterv1.GroupVersion.WithKind(clusterv1.ClusterKind))
	return c
}

// unstructuredCluster returns a Cluster written with an apiVersion this
// installer has no compiled-in type for, which is what current upstream
// Cluster API documentation shows.
func unstructuredCluster() client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("cluster.x-k8s.io/v1beta2")
	u.SetKind("Cluster")
	u.SetName("test-cluster")
	return u
}

// unstructuredMachine returns a Machine at a version this installer has no
// compiled-in type for.
func unstructuredMachine() client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("cluster.x-k8s.io/v1beta2")
	u.SetKind("Machine")
	u.SetName("test-bootstrap")
	return u
}

func namespaceObject() client.Object {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-cluster-api-guests"}}
	ns.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	return ns
}

func machineObject() client.Object {
	m := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "test-bootstrap"}}
	m.SetGroupVersionKind(clusterv1.GroupVersion.WithKind("Machine"))
	return m
}

// TestValidateManifests covers the check that turns the worst failure mode on
// this path -- an install that provisions nothing and reports success -- into
// an error the user can act on.
func TestValidateManifests(t *testing.T) {
	cases := []struct {
		name     string
		infra    []client.Object
		machines []client.Object
		wantErr  string
	}{
		{
			name:     "no manifests at all",
			machines: []client.Object{machineObject()},
			wantErr:  "no Cluster object found",
		},
		{
			name:     "infrastructure object but no cluster",
			infra:    []client.Object{infraObject()},
			machines: []client.Object{machineObject()},
			wantErr:  "no Cluster object found",
		},
		{
			name:     "cluster but no infrastructure object",
			infra:    []client.Object{typedCluster()},
			machines: []client.Object{machineObject()},
			wantErr:  "no infrastructure object found",
		},
		{
			// The namespace is supplied by the installer, not the user.
			// Counting it would let a set with no infrastructure object pass.
			name:     "namespace does not count as the infrastructure object",
			infra:    []client.Object{typedCluster(), namespaceObject()},
			machines: []client.Object{machineObject()},
			wantErr:  "no infrastructure object found",
		},
		{
			// An infrastructure-only run. With no Machine the install cannot
			// complete, but it is the cheap way to exercise a provider, so it
			// warns rather than refusing to start.
			name:  "no machines is allowed",
			infra: []client.Object{typedCluster(), infraObject()},
		},
		{
			name:     "cluster, infrastructure object and machine",
			infra:    []client.Object{typedCluster(), infraObject()},
			machines: []client.Object{machineObject()},
		},
		{
			// Recognised, and then rejected. An earlier revision accepted this
			// so that the error would not be a misleading "no Cluster object
			// found" while one sat in the directory. Accepting it is worse:
			// provisioning finds Clusters by their compiled-in type, so this
			// one is created and never waited for, and the install reports the
			// infrastructure ready without having waited for anything. The
			// recognition is kept; the outcome is now an error that names the
			// version to use.
			name:     "cluster in another api version is rejected, by version",
			infra:    []client.Object{unstructuredCluster(), infraObject()},
			machines: []client.Object{machineObject()},
			wantErr:  "cluster.x-k8s.io/v1beta1",
		},
		{
			// The same skew on a Machine, which has the same type assertion at
			// pkg/infrastructure/clusterapi/clusterapi.go:387.
			name:     "machine in another api version is rejected",
			infra:    []client.Object{typedCluster(), infraObject()},
			machines: []client.Object{unstructuredMachine()},
			wantErr:  "cluster.x-k8s.io/v1beta1",
		},
		{
			// The provider's own object is not core Cluster API and is not
			// version-checked: the installer does not know its schema, and
			// pinning a version it has no type for would be meaningless.
			name:     "the infrastructure object's version is not constrained",
			infra:    []client.Object{typedCluster(), infraObjectAtVersion("v1alpha7")},
			machines: []client.Object{machineObject()},
		},
		{
			name:     "namespace alongside a complete set is fine",
			infra:    []client.Object{namespaceObject(), typedCluster(), infraObject()},
			machines: []client.Object{machineObject()},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Provider{}.ValidateManifests(tc.infra, tc.machines)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			// The message has to say where to put the files, or it is not
			// actionable.
			assert.Contains(t, err.Error(), externaltypes.ManifestDir)
		})
	}
}

// writeManifest creates a manifest file under dir, creating dir first.
func writeManifest(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

const clusterYAML = `apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
`

const infraYAML = `apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: SomeCloudCluster
metadata:
  name: test-cluster
`

const machineYAML = `apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: test-bootstrap
`

// TestProvideManifests covers reading the user's manifests out of the install
// directory. This is the External platform's entire input: the installer
// generates none of it.
func TestProvideManifests(t *testing.T) {
	t.Run("reads infrastructure and machine objects", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, externaltypes.ManifestDir)
		writeManifest(t, root, "cluster.yaml", clusterYAML)
		writeManifest(t, root, "infra.yaml", infraYAML)
		writeManifest(t, filepath.Join(root, externaltypes.MachineManifestDir), "bootstrap.yaml", machineYAML)

		infra, machines, err := Provider{}.ProvideManifests(context.Background(), dir)
		require.NoError(t, err)
		assert.Len(t, infra, 2)
		require.Len(t, machines, 1)
		assert.Equal(t, "test-bootstrap", machines[0].GetName())

		// The pair the user is most likely to get wrong: the Cluster must
		// come back as the typed object, because the installer waits on its
		// status, while the provider's own object stays unstructured.
		assert.IsType(t, &clusterv1.Cluster{}, infra[0])
		assert.IsType(t, &unstructured.Unstructured{}, infra[1])
	})

	t.Run("machines are optional", func(t *testing.T) {
		// An infrastructure-only run: no machines subdirectory at all.
		dir := t.TempDir()
		writeManifest(t, filepath.Join(dir, externaltypes.ManifestDir), "cluster.yaml", clusterYAML)

		infra, machines, err := Provider{}.ProvideManifests(context.Background(), dir)
		require.NoError(t, err)
		assert.Len(t, infra, 1)
		assert.Empty(t, machines)
	})

	t.Run("a missing directory is an error that names it", func(t *testing.T) {
		// Reaching here means the install-config asked for Cluster API
		// provisioning, so an absent directory is a mistake, not an
		// instruction to provision nothing.
		_, _, err := Provider{}.ProvideManifests(context.Background(), t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), externaltypes.ManifestDir)
	})

	t.Run("several objects in one file are all read", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, filepath.Join(dir, externaltypes.ManifestDir), "both.yaml",
			clusterYAML+"---\n"+infraYAML)

		infra, _, err := Provider{}.ProvideManifests(context.Background(), dir)
		require.NoError(t, err)
		assert.Len(t, infra, 2)
	})

	t.Run("non-manifest files and subdirectories are ignored", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, externaltypes.ManifestDir)
		writeManifest(t, root, "cluster.yaml", clusterYAML)
		writeManifest(t, root, "notes.txt", "not a manifest")
		writeManifest(t, root, "README.md", "# not a manifest")
		writeManifest(t, filepath.Join(root, "scratch"), "ignored.yaml", infraYAML)

		infra, _, err := Provider{}.ProvideManifests(context.Background(), dir)
		require.NoError(t, err)
		assert.Len(t, infra, 1)
	})

	t.Run("a malformed manifest names the file", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, filepath.Join(dir, externaltypes.ManifestDir), "broken.yaml", "\tnot: [valid")

		_, _, err := Provider{}.ProvideManifests(context.Background(), dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "broken.yaml")
	})
}

// TestProvideThenValidate runs the two hooks in the order Provision runs
// them, against manifests read from disk.
//
// The hooks were only ever tested apart, with ValidateManifests fed objects
// built in the test and given their GroupVersionKind explicitly. Decoding a
// manifest does not leave the GVK set on a typed object, so every such test
// passed while the real path failed: an install with a perfectly good Cluster
// on disk was rejected for not having one. Anything that reads a manifest and
// then judges it has to be tested across that seam.
func TestProvideThenValidate(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, externaltypes.ManifestDir)
	writeManifest(t, root, "cluster.yaml", clusterYAML+"---\n"+infraYAML)
	writeManifest(t, filepath.Join(root, externaltypes.MachineManifestDir), "bootstrap.yaml", machineYAML)

	infra, machines, err := Provider{}.ProvideManifests(context.Background(), dir)
	require.NoError(t, err)
	assert.NoError(t, Provider{}.ValidateManifests(infra, machines))
}

// TestProvideThenValidateRejectsVersionSkew is the same seam, for the check
// that a core object at another version is refused rather than silently
// ignored. It has to go through the decoder too: a skewed Cluster comes back
// unstructured, which is a different branch from the typed one above.
func TestProvideThenValidateRejectsVersionSkew(t *testing.T) {
	dir := t.TempDir()
	skewed := strings.Replace(clusterYAML, "v1beta1", "v1beta2", 1)
	writeManifest(t, filepath.Join(dir, externaltypes.ManifestDir), "cluster.yaml", skewed+"---\n"+infraYAML)

	infra, machines, err := Provider{}.ProvideManifests(context.Background(), dir)
	require.NoError(t, err)
	err = Provider{}.ValidateManifests(infra, machines)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster.x-k8s.io/v1beta1")
}

func TestProviderIdentity(t *testing.T) {
	assert.Equal(t, "external", Provider{}.Name())
	assert.Equal(t, "InternalIP", string(Provider{}.PublicGatherEndpoint()))
	assert.True(t, Provider{}.TolerateUnstructuredManifests())
}
