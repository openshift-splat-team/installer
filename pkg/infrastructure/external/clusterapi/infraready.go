package clusterapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/installer/cmd/openshift-install/command"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/manifests/capiutils"
	infracapi "github.com/openshift/installer/pkg/infrastructure/clusterapi"
	"github.com/openshift/installer/pkg/infrastructure/external/hooks"
	externaltypes "github.com/openshift/installer/pkg/types/external"
)

// InfraReady runs the user's infra-ready hook, if they configured one.
//
// This is where an integrated platform creates the DNS its cluster cannot
// boot without. The External platform has no such code and must not acquire
// any, so the work is handed to a program the user supplies -- see
// pkg/infrastructure/external/hooks for why that seam exists and what it is
// given.
//
// The timing is not negotiable. It has to be after the provider reports the
// infrastructure ready, because only then do the load balancers a DNS record
// must point at exist; and it has to be before machines are created, because
// a control-plane machine fetches its ignition from api-int and a bootstrap
// node resolves api-int on its first reconcile. The InfraReady hook is the
// only point that satisfies both, which is why the installer's integrated
// providers use it for exactly the same purpose.
func (p Provider) InfraReady(ctx context.Context, in infracapi.InfraReadyInput) error {
	configured := configuredHooks(in.InstallConfig)
	if configured == nil || configured.InfraReady == "" {
		warnNoInfraReadyHook(in.InstallConfig)
		return nil
	}

	cluster, err := coreCluster(ctx, in.Client, in.InfraID)
	if err != nil {
		return err
	}

	req := hooks.Request{
		Kind:                     hooks.InfraReady,
		Program:                  configured.InfraReady,
		InstallDir:               command.RootOpts.Dir,
		InfraID:                  in.InfraID,
		ClusterName:              in.InstallConfig.Config.ObjectMeta.Name,
		BaseDomain:               in.InstallConfig.Config.BaseDomain,
		Publish:                  string(in.InstallConfig.Config.Publish),
		ControlPlaneEndpointHost: cluster.Spec.ControlPlaneEndpoint.Host,
		ControlPlaneEndpointPort: strconv.Itoa(int(cluster.Spec.ControlPlaneEndpoint.Port)),
	}

	if req.ClusterJSON, err = json.Marshal(cluster); err != nil {
		return fmt.Errorf("failed to serialise the Cluster for the hook: %w", err)
	}
	// The provider's own object is best-effort. A hook that needs it will
	// fail on its own and say why, in its own terms; failing here would mean
	// the installer deciding that an object it cannot interpret is required.
	if infra, err := infrastructureObject(ctx, in.Client, cluster); err != nil {
		logrus.Warnf("Could not read the infrastructure object to pass to the hook, so only the "+
			"cluster identity and the control plane endpoint will be available to it: %v", err)
	} else if infra != nil {
		if req.InfraJSON, err = infra.MarshalJSON(); err != nil {
			return fmt.Errorf("failed to serialise the infrastructure object for the hook: %w", err)
		}
	}

	return hooks.Run(ctx, req)
}

// configuredHooks returns the hooks the install-config names, or nil.
func configuredHooks(ic *installconfig.InstallConfig) *externaltypes.Hooks {
	if ic == nil || ic.Config == nil || ic.Config.Platform.External == nil {
		return nil
	}
	capi := ic.Config.Platform.External.ClusterAPI
	if capi == nil {
		return nil
	}
	return capi.Hooks
}

// warnNoInfraReadyHook says what will go wrong, rather than that something is
// missing.
//
// Running no hook is a supported configuration: the records may be created
// out of band, which is what the non-Cluster-API External CI does with
// CloudFormation, and refusing to proceed would break it. But the failure
// that follows when nothing creates them is remote from its cause -- the
// install runs to the end of the bootstrap timeout and dies inside a bootkube
// stage -- so the warning names that stage by the string the user will
// actually see in the error.
func warnNoInfraReadyHook(ic *installconfig.InstallConfig) {
	domain := "the cluster domain"
	if ic != nil && ic.Config != nil {
		domain = ic.Config.ClusterDomain()
	}
	logrus.Warnf("No platform.external.clusterAPI.hooks.infraReady is configured, so the installer is "+
		"creating no DNS for this cluster. api-int.%s must resolve to the internal API load balancer before "+
		"the control plane can start: the bootstrap node resolves it to reach its own API server, and every "+
		"control-plane machine fetches its ignition from it on port 22623. If nothing else creates that "+
		"record, this install will run to the end of the bootstrap timeout and then fail at the bootkube "+
		"stage \"resolve-api-int-url\".", domain)
}

// coreCluster reads the Cluster whose status the installer just waited on.
//
// It is looked up by listing rather than by name because the name belongs to
// the user's manifest: the installer waits on whatever Cluster objects the
// manifests declared, and does not require any of them to be called after the
// infrastructure ID. Preferring an exact match on the infrastructure ID and
// falling back to a sole Cluster covers both the convention and the general
// case, and anything else is ambiguous enough to refuse.
func coreCluster(ctx context.Context, cl client.Client, infraID string) (*clusterv1.Cluster, error) {
	list := &clusterv1.ClusterList{}
	if err := cl.List(ctx, list, client.InNamespace(capiutils.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list Cluster objects: %w", err)
	}

	switch {
	case len(list.Items) == 0:
		return nil, fmt.Errorf("no %s object found in the %s namespace after the infrastructure reported ready",
			clusterv1.ClusterKind, capiutils.Namespace)
	case len(list.Items) == 1:
		return &list.Items[0], nil
	}
	for i := range list.Items {
		if list.Items[i].Name == infraID {
			return &list.Items[i], nil
		}
	}
	return nil, fmt.Errorf("found %d %s objects in the %s namespace and none named %q, so the installer "+
		"cannot tell which one the hook should be told about",
		len(list.Items), clusterv1.ClusterKind, capiutils.Namespace, infraID)
}

// infrastructureObject reads the provider's own object, unstructured.
//
// Unstructured is not a fallback here, it is the only possibility: the object
// is the provider's API and this binary has no Go type for it. Reading it
// through the reference on the Cluster rather than by guessing a kind is what
// keeps this provider-agnostic -- the core Cluster API contract guarantees
// the reference resolves, and says nothing about what it resolves to.
func infrastructureObject(ctx context.Context, cl client.Client, cluster *clusterv1.Cluster) (*unstructured.Unstructured, error) {
	ref := cluster.Spec.InfrastructureRef
	if ref == nil {
		return nil, fmt.Errorf("%s %s has no spec.infrastructureRef", clusterv1.ClusterKind, cluster.Name)
	}

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the infrastructureRef apiVersion %q: %w", ref.APIVersion, err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gv.WithKind(ref.Kind))

	namespace := ref.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	if err := cl.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, obj); err != nil {
		return nil, fmt.Errorf("failed to read %s %s/%s: %w", ref.Kind, namespace, ref.Name, err)
	}
	return obj, nil
}
