package external

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	capav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/installer/pkg/clusterapi"
)

// recordingClient records every write in order. It embeds client.Client as a
// nil interface so that any method the destroyer calls but the test has not
// modelled panics rather than silently returning a zero value.
type recordingClient struct {
	client.Client

	// calls is the ordered log, one entry per write, as "verb Kind/name".
	calls []string
	// created is the object as it was handed to Create, before the API
	// server would have touched it.
	created map[string]client.Object
	// gone is the set of keys Get should report as deleted.
	gone map[string]bool
	// getObjects answers Get for keys that exist.
	getObjects map[string]client.Object
}

func newRecordingClient() *recordingClient {
	return &recordingClient{
		created:    map[string]client.Object{},
		gone:       map[string]bool{},
		getObjects: map[string]client.Object{},
	}
}

func key(obj client.Object) string {
	return fmt.Sprintf("%s/%s", kindOf(obj), obj.GetName())
}

func (c *recordingClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	c.calls = append(c.calls, "create "+key(obj))
	c.created[key(obj)] = copyObject(obj)
	// Stand in for what the API server assigns.
	obj.SetResourceVersion("1")
	obj.SetUID(apitypes.UID("uid-" + obj.GetName()))
	return nil
}

func (c *recordingClient) Get(_ context.Context, k client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	id := fmt.Sprintf("%s/%s", kindOf(obj), k.Name)
	if c.gone[id] {
		return apierrors.NewNotFound(schema.GroupResource{Resource: kindOf(obj)}, k.Name)
	}
	stored, ok := c.getObjects[id]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{Resource: kindOf(obj)}, k.Name)
	}
	switch target := obj.(type) {
	case *clusterv1.Cluster:
		//nolint:forcetypeassert // the test only stores a Cluster under a Cluster key.
		stored.(*clusterv1.Cluster).DeepCopyInto(target)
	default:
		return fmt.Errorf("recordingClient cannot Get into %T", obj)
	}
	return nil
}

func (c *recordingClient) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	c.calls = append(c.calls, "delete "+key(obj))
	c.gone[key(obj)] = true
	return nil
}

func (c *recordingClient) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	c.calls = append(c.calls, "patch "+key(obj))
	return nil
}

func (c *recordingClient) Status() client.SubResourceWriter {
	return &recordingStatusWriter{parent: c}
}

type recordingStatusWriter struct {
	client.SubResourceWriter
	parent *recordingClient
}

func (w *recordingStatusWriter) Update(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	w.parent.calls = append(w.parent.calls, "status "+key(obj))
	return nil
}

func testUninstaller(cl *recordingClient) *ClusterUninstaller { //nolint:unparam // symmetry with the other helpers.
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.PanicLevel)
	return &ClusterUninstaller{Logger: logger, Dir: "/does/not/matter"}
}

// fixture builds the three objects a provisioned External cluster leaves
// behind: the guests namespace, an infrastructure cluster, and the Cluster
// that points at it.
func fixture() (namespace, infra, cluster client.Object) {
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "openshift-cluster-api-guests"},
	}
	infra = &capav1.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cluster",
			Namespace:       "openshift-cluster-api-guests",
			ResourceVersion: "4242",
			UID:             "stale-uid",
			Finalizers:      []string{"awscluster.infrastructure.cluster.x-k8s.io"},
			OwnerReferences: []metav1.OwnerReference{{Name: "test-cluster", UID: "stale-owner"}},
		},
		Status: capav1.AWSClusterStatus{
			Ready: true,
			Network: capav1.NetworkStatus{
				SecurityGroups: map[capav1.SecurityGroupRole]capav1.SecurityGroup{},
			},
		},
	}
	cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cluster",
			Namespace:       "openshift-cluster-api-guests",
			ResourceVersion: "4243",
			UID:             "stale-uid",
			Finalizers:      []string{clusterv1.ClusterFinalizer},
		},
		Status: clusterv1.ClusterStatus{InfrastructureReady: true},
	}
	return namespace, infra, cluster
}

// TestRestoreOrdersAndPauses pins the whole restore contract in one place,
// because every part of it exists to stop the provider building a second copy
// of the infrastructure.
func TestRestoreOrdersAndPauses(t *testing.T) {
	namespace, infra, cluster := fixture()
	cl := newRecordingClient()
	u := testUninstaller(cl)

	// Deliberately out of dependency order: restore must impose its own.
	restored, err := u.restore(context.Background(), cl, []client.Object{cluster, infra, namespace})
	require.NoError(t, err)
	require.Len(t, restored, 3)

	assert.Equal(t, []string{
		"create Namespace/openshift-cluster-api-guests",
		"create AWSCluster/test-cluster",
		"status AWSCluster/test-cluster",
		"create Cluster/test-cluster",
		"status Cluster/test-cluster",
	}, cl.calls, "namespace, then infrastructure, then the Cluster that references it; "+
		"and each status written back before the next object appears")

	for _, name := range []string{"AWSCluster/test-cluster", "Cluster/test-cluster"} {
		created := cl.created[name]
		assert.Equal(t, "true", created.GetAnnotations()[clusterv1.PausedAnnotation],
			"%s must be created paused, or the provider reconciles it before its status is restored", name)
		assert.Empty(t, created.GetResourceVersion(), "%s: a stale resourceVersion is rejected by Create", name)
		assert.Empty(t, created.GetUID(), "%s: a stale uid is rejected by Create", name)
		assert.Empty(t, created.GetOwnerReferences(), "%s: owner references point at uids that no longer exist", name)
	}

	assert.True(t, cl.created["Cluster/test-cluster"].(*clusterv1.Cluster).Spec.Paused, //nolint:forcetypeassert // fixture type.
		"the core Cluster controller reads spec.paused, not the annotation")

	assert.Equal(t, []string{"awscluster.infrastructure.cluster.x-k8s.io"},
		cl.created["AWSCluster/test-cluster"].GetFinalizers(),
		"finalizers must survive the restore: a Cluster deleted before its controller re-adds one "+
			"is removed immediately and destroy reports success having deleted nothing")
	assert.Equal(t, []string{clusterv1.ClusterFinalizer}, cl.created["Cluster/test-cluster"].GetFinalizers())
}

// TestUnpauseClearsBothMechanisms checks that handing the objects back is the
// exact inverse of pausing them. A Cluster left paused is never reconciled,
// so the delete would hang for the full timeout.
func TestUnpauseClearsBothMechanisms(t *testing.T) {
	_, infra, cluster := fixture()
	pause(infra)
	pause(cluster)

	cl := newRecordingClient()
	u := testUninstaller(cl)
	require.NoError(t, u.unpause(context.Background(), cl, []client.Object{infra, cluster}))

	assert.Equal(t, []string{"patch AWSCluster/test-cluster", "patch Cluster/test-cluster"}, cl.calls)
	assert.NotContains(t, infra.GetAnnotations(), clusterv1.PausedAnnotation)
	assert.NotContains(t, cluster.GetAnnotations(), clusterv1.PausedAnnotation)
	//nolint:forcetypeassert // fixture type.
	assert.False(t, cluster.(*clusterv1.Cluster).Spec.Paused)
}

// TestUnpauseSkipsNamespace guards against patching a cluster-scoped object
// that was never paused in the first place.
func TestUnpauseSkipsNamespace(t *testing.T) {
	namespace, _, _ := fixture()
	cl := newRecordingClient()
	u := testUninstaller(cl)

	require.NoError(t, u.unpause(context.Background(), cl, []client.Object{namespace}))
	assert.Empty(t, cl.calls)
}

// TestDeleteClusterRefusesWithoutFinalizer is the most important test here.
// Without the guard, deleting a Cluster that no controller has claimed
// succeeds instantly, the wait sees it gone, and destroy reports success
// while the entire cloud footprint is still running and still billing.
func TestDeleteClusterRefusesWithoutFinalizer(t *testing.T) {
	_, _, cluster := fixture()
	cluster.SetFinalizers(nil)

	cl := newRecordingClient()
	cl.getObjects["Cluster/test-cluster"] = cluster
	u := testUninstaller(cl)

	err := u.deleteCluster(context.Background(), cl, cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no finalizer")
	assert.Contains(t, err.Error(), "still running")
	assert.Empty(t, cl.calls, "nothing may be deleted once the guard trips")
}

// TestDeleteClusterWaitsForDisappearance confirms the success path deletes
// and then observes the object gone, which is the provider's own report that
// it finished.
func TestDeleteClusterWaitsForDisappearance(t *testing.T) {
	_, _, cluster := fixture()
	cl := newRecordingClient()
	cl.getObjects["Cluster/test-cluster"] = cluster
	u := testUninstaller(cl)

	require.NoError(t, u.deleteCluster(context.Background(), cl, cluster))
	assert.Equal(t, []string{"delete Cluster/test-cluster"}, cl.calls)
}

// TestDeleteClusterReportsMissingCluster covers an install directory whose
// artifacts name a Cluster the control plane does not have. Returning nil
// here would be reported to the user as a successful destroy.
func TestDeleteClusterReportsMissingCluster(t *testing.T) {
	_, _, cluster := fixture()
	cl := newRecordingClient()
	u := testUninstaller(cl)

	err := u.deleteCluster(context.Background(), cl, cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before deleting it")
}

func TestLoadArtifacts(t *testing.T) {
	const clusterYAML = `apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
  namespace: openshift-cluster-api-guests
  finalizers:
  - cluster.cluster.x-k8s.io
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
    kind: AWSCluster
    name: test-cluster
`
	// A kind no installer type exists for, which is the ordinary case for a
	// provider the installer was never compiled against.
	const unknownYAML = `apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCICluster
metadata:
  name: test-cluster
  namespace: openshift-cluster-api-guests
spec:
  compartmentId: ocid1.compartment.oc1..example
`

	t.Run("decodes both typed and unknown kinds", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Cluster-ns-test.yaml"), []byte(clusterYAML), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "OCICluster-ns-test.yaml"), []byte(unknownYAML), 0o600))

		objects, err := loadArtifacts(dir)
		require.NoError(t, err)
		require.Len(t, objects, 2)

		clusters := filter(objects, isCoreCluster)
		require.Len(t, clusters, 1)
		assert.Equal(t, []string{"cluster.cluster.x-k8s.io"}, clusters[0].GetFinalizers(),
			"the recorded finalizer is what makes the delete meaningful")

		unknown := filter(objects, func(o client.Object) bool { _, ok := o.(*unstructured.Unstructured); return ok })
		require.Len(t, unknown, 1, "a kind with no compiled-in type must be carried through, not rejected")
		assert.Equal(t, "OCICluster", kindOf(unknown[0]))
	})

	t.Run("missing directory is named in the error", func(t *testing.T) {
		_, err := loadArtifacts(filepath.Join(t.TempDir(), "absent"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "absent")
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("empty directory is not a silent success", func(t *testing.T) {
		_, err := loadArtifacts(t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no manifests found")
	})
}

// TestKindOfHandlesEmptyTypeMeta covers the trap that decoding through the
// scheme leaves TypeMeta empty, so an object cannot be identified by the GVK
// it carries. isNamespace, and therefore the whole restore ordering, depends
// on this.
func TestKindOfHandlesEmptyTypeMeta(t *testing.T) {
	namespace, infra, cluster := fixture()
	for _, obj := range []client.Object{namespace, infra, cluster} {
		require.Empty(t, obj.GetObjectKind().GroupVersionKind().Kind,
			"the fixture must reproduce the empty TypeMeta a decoded object has")
	}

	assert.Equal(t, "Namespace", kindOf(namespace))
	assert.Equal(t, "AWSCluster", kindOf(infra))
	assert.Equal(t, "Cluster", kindOf(cluster))

	assert.True(t, isNamespace(namespace))
	assert.False(t, isNamespace(infra))
	assert.False(t, isCoreCluster(infra))
	assert.True(t, isCoreCluster(cluster))

	// An unstructured object carries its own GVK and must not need the scheme.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Kind: "OCICluster"})
	assert.Equal(t, "OCICluster", kindOf(u))
	assert.False(t, isCoreCluster(u), "an unstructured Cluster-like object is deliberately not treated as a Cluster")
}

// TestSchemeKnowsFixtureKinds guards the assumption kindOf rests on: the
// local control plane's scheme can name these Go types.
func TestSchemeKnowsFixtureKinds(t *testing.T) {
	namespace, infra, cluster := fixture()
	for _, obj := range []client.Object{namespace, infra, cluster} {
		gvks, _, err := clusterapi.Scheme.ObjectKinds(obj)
		require.NoError(t, err, "%T is not registered in the local control plane scheme", obj)
		require.NotEmpty(t, gvks)
	}
}
