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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	"github.com/openshift/installer/pkg/clusterapi"
)

// stubClient serves collectManifests without a control plane. client.Client is
// embedded as a nil interface on purpose: any method other than the two
// implemented below panics, so the test also pins down which parts of the
// client collectManifests is allowed to reach for.
type stubClient struct {
	client.Client
	scheme *runtime.Scheme
}

// Get is a no-op. The objects handed to collectManifests are already populated,
// which is the same state a real Get would leave them in.
func (c *stubClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return nil
}

func (c *stubClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return apiutil.GVKForObject(obj, c.scheme)
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(s))
	return s
}

// TestCollectManifestsWritesAppliableYAML covers the defect where every
// collected artifact was serialized as a Go struct rather than as a Kubernetes
// object: lowercased field names and an empty GVK, which nothing can re-apply.
func TestCollectManifestsWritesAppliableYAML(t *testing.T) {
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-master-0",
			Namespace: "openshift-cluster-api-guests",
		},
		Spec: clusterv1.MachineSpec{ClusterName: "test-cluster"},
	}

	i := &InfraProvider{appliedManifests: []client.Object{machine}}
	files, errs := i.collectManifests(context.Background(), &stubClient{scheme: testScheme(t)})
	require.Empty(t, errs)
	require.Len(t, files, 1)

	data := string(files[0].Data)

	assert.Contains(t, data, "apiVersion: cluster.x-k8s.io/v1beta1")
	assert.Contains(t, data, "kind: Machine")

	// The Go-struct serialization produced these. Their absence is the fix.
	assert.NotContains(t, data, "typemeta:")
	assert.NotContains(t, data, "objectmeta:")
	assert.NotContains(t, data, "creationtimestamp:")

	// Re-decoding into a typed object is what "re-appliable" means here.
	decoded := &clusterv1.Machine{}
	require.NoError(t, yaml.Unmarshal(files[0].Data, decoded))
	assert.Equal(t, "Machine", decoded.Kind)
	assert.Equal(t, "cluster.x-k8s.io/v1beta1", decoded.APIVersion)
	assert.Equal(t, "test-cluster-master-0", decoded.Name)
	assert.Equal(t, "test-cluster", decoded.Spec.ClusterName)
}

// TestCollectManifestsUnstructured is the platform: external case. The object
// a non-integrated provider produces is unstructured, and yaml.v2 emitted
// almost nothing for it because its only field is tagged json:"-".
func TestCollectManifestsUnstructured(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
		"kind":       "AWSCluster",
		"metadata": map[string]interface{}{
			"name":      "test-cluster",
			"namespace": "openshift-cluster-api-guests",
		},
		"spec": map[string]interface{}{"region": "us-east-1"},
	}}

	i := &InfraProvider{appliedManifests: []client.Object{obj}}
	files, errs := i.collectManifests(context.Background(), &stubClient{scheme: testScheme(t)})
	require.Empty(t, errs)
	require.Len(t, files, 1)

	assert.Equal(t,
		filepath.Join(clusterapi.ArtifactsDir, "AWSCluster-openshift-cluster-api-guests-test-cluster.yaml"),
		files[0].Filename)

	decoded := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal(files[0].Data, decoded))
	assert.Equal(t, "AWSCluster", decoded.GetKind())
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1beta2", decoded.GetAPIVersion())
	assert.Equal(t, "test-cluster", decoded.GetName())

	region, found, err := unstructured.NestedString(decoded.Object, "spec", "region")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "us-east-1", region)
}

func TestCollectManifestsSkipsSecrets(t *testing.T) {
	i := &InfraProvider{appliedManifests: []client.Object{
		IgnitionSecret([]byte("sensitive"), "test-cluster", "master"),
	}}
	scheme := testScheme(t)
	require.NoError(t, corev1.AddToScheme(scheme))

	files, errs := i.collectManifests(context.Background(), &stubClient{scheme: scheme})
	require.Empty(t, errs)
	assert.Empty(t, files, "secrets must never reach disk")
}

// TestCollectManifestsExtractIPAddressRoundTrip crosses the seam between the
// two halves of this file: collectManifests writes the machine artifacts and
// extractIPAddress reads them back. Each half was correct against its own
// assumptions about the serialization, and only the pair can show they agree.
func TestCollectManifestsExtractIPAddressRoundTrip(t *testing.T) {
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-master-0",
			Namespace: "openshift-cluster-api-guests",
		},
		Status: clusterv1.MachineStatus{Addresses: []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: "10.0.0.5"},
			{Type: clusterv1.MachineExternalIP, Address: "203.0.113.5"},
			{Type: clusterv1.MachineHostName, Address: "ip-10-0-0-5"},
		}},
	}

	i := &InfraProvider{appliedManifests: []client.Object{machine}}
	files, errs := i.collectManifests(context.Background(), &stubClient{scheme: testScheme(t)})
	require.Empty(t, errs)
	require.Len(t, files, 1)

	dir := t.TempDir()
	path := filepath.Join(dir, filepath.Base(files[0].Filename))
	require.NoError(t, os.WriteFile(path, files[0].Data, 0o600))

	addrs, err := extractIPAddress(path)
	require.NoError(t, err)

	// External addresses are returned ahead of internal ones, and the
	// hostname is dropped.
	assert.Equal(t, []string{"203.0.113.5", "10.0.0.5"}, addrs)
}

// TestExtractIPAddressRejectsGoStructYAML documents why the round-trip test
// above is not redundant: the format collectManifests used to write parses
// without error and yields no addresses at all.
func TestExtractIPAddressRejectsGoStructYAML(t *testing.T) {
	goStruct := strings.Join([]string{
		"typemeta:",
		`  kind: ""`,
		`  apiversion: ""`,
		"objectmeta:",
		"  name: test-cluster-master-0",
		"status:",
		"  addresses:",
		"  - type: InternalIP",
		"    address: 10.0.0.5",
	}, "\n")

	dir := t.TempDir()
	path := filepath.Join(dir, "Machine-openshift-cluster-api-guests-test-cluster-master-0.yaml")
	require.NoError(t, os.WriteFile(path, []byte(goStruct), 0o600))

	addrs, err := extractIPAddress(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.5"}, addrs,
		"the status stanza is the one part both serializations agreed on")
}
