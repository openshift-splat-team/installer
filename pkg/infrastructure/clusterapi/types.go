package clusterapi

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/installer/pkg/asset/cluster/tfvars"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/machines"
	"github.com/openshift/installer/pkg/asset/manifests"
	"github.com/openshift/installer/pkg/asset/rhcos"
	"github.com/openshift/installer/pkg/asset/tls"
	"github.com/openshift/installer/pkg/types"
)

// Provider is the base interface that cloud platforms
// should implement for the CAPI infrastructure provider.
type Provider interface {
	// Name provides the name for the cloud platform.
	Name() string

	// PublicGatherEndpoint returns how the cloud platform expects the installer
	// to connect to the bootstrap node for log gathering. CAPI providers are not
	// consistent in how public IP addresses are represented in the machine status.
	// Furthermore, Azure cannot attach a public IP to the bootstrap node, so SSH
	// must be performed through the API load balancer.
	// When a platform returns ExternalIP, the installer will require an ExternalIP
	// to be present in the status, before it declares the machine ready.
	PublicGatherEndpoint() GatherEndpoint
}

// ManifestValidator defines the ValidateManifests hook, which is called with
// the Cluster API objects the installer is about to create, before any
// provisioning has begun.
//
// It exists for providers whose manifests the installer does not generate. For
// those, an empty or incomplete set of manifests is not an internal error but
// an ordinary user mistake, and it would otherwise produce an install that
// provisions nothing and reports no error.
type ManifestValidator interface {
	// ValidateManifests is called with the infrastructure and machine
	// manifests collected for the cluster, before the local control plane is
	// started. Machines are passed separately because for a platform whose
	// manifests the installer does not generate, an empty machine set is
	// itself a user mistake that would otherwise provision nothing silently.
	ValidateManifests(infra, machines []client.Object) error
}

// UnstructuredManifestTolerator marks a provider that accepts manifests whose
// kind this installer has no compiled-in type for.
//
// Only a platform whose infrastructure provider is supplied by the user can
// legitimately reference such a kind: its CRDs are installed into the local
// control plane from the provider's own components, so the API server can
// serve a type this binary was never built against.
//
// For every compiled-in platform an unresolvable kind means a mistake -- a
// misspelled kind, or a wrong apiVersion -- and Provision rejects it before any
// cloud resource is created. That preserves the load-time strictness those
// platforms have always had, which the unstructured fallback in
// pkg/clusterapi.ObjectsFromManifest would otherwise have given up for
// everyone.
type UnstructuredManifestTolerator interface {
	// TolerateUnstructuredManifests reports whether unresolved kinds are
	// expected for this platform.
	TolerateUnstructuredManifests() bool
}

// ManifestProvider defines the ProvideManifests hook, which lets a provider
// contribute Cluster API objects that the installer did not generate.
//
// Every compiled-in platform has a manifest generator, so its objects reach
// Provision through the manifest assets and it has no reason to implement
// this. A platform whose infrastructure provider is supplied by the user has
// no generator -- the installer does not know the provider's API -- so the
// objects come from files the user wrote, read by the provider itself.
//
// Reading them here rather than through a WritableAsset's Load is deliberate.
// Assets loaded from the install directory are discarded when their
// dependencies are dirty (pkg/asset/store/store.go), which happens whenever
// install-config.yaml is still present, so user files on that path disappear
// with only a warning. This hook runs at Provision, after the asset graph has
// been resolved, and is not subject to that rule.
//
// The objects returned are appended to those from the assets, so anything the
// installer establishes first -- the namespace everything is created into,
// above all -- still comes first. They are then validated and created exactly
// like generated ones: there is a single create path for every platform.
type ManifestProvider interface {
	// ProvideManifests returns infrastructure and machine objects to create
	// in addition to those produced by the manifest assets. installDir is the
	// install directory, the root the provider resolves user paths against.
	ProvideManifests(ctx context.Context, installDir string) (infra, machines []client.Object, err error)
}

// PreProvider defines the PreProvision hook, which is called prior to
// CAPI infrastructure provisioning.
type PreProvider interface {
	// PreProvision is called before provisioning using CAPI controllers has begun
	// and should be used to create dependencies needed for CAPI provisioning,
	// such as IAM roles or policies.
	PreProvision(ctx context.Context, in PreProvisionInput) error
}

// PreProvisionInput collects the args passed to the PreProvision call.
type PreProvisionInput struct {
	InfraID          string
	InstallConfig    *installconfig.InstallConfig
	RhcosImage       *rhcos.Image
	ManifestsAsset   *manifests.Manifests
	MachineManifests []client.Object
	WorkersAsset     *machines.Worker
}

// IgnitionProvider handles preconditions for bootstrap ignition,
// such as pushing to cloud storage. Returns bootstrap and master
// ignition secrets.
//
// WARNING! Low-level primitive. Use only if absolutely necessary.
type IgnitionProvider interface {
	Ignition(ctx context.Context, in IgnitionInput) ([]*corev1.Secret, error)
}

// IgnitionInput collects the args passed to the IgnitionProvider call.
type IgnitionInput struct {
	Client           client.Client
	BootstrapIgnData []byte
	MasterIgnData    []byte
	WorkerIgnData    []byte
	InfraID          string
	InstallConfig    *installconfig.InstallConfig
	TFVarsAsset      *tfvars.TerraformVariables
	RootCA           *tls.RootCA
}

// WithOutput returns a new IgnitionInput with ignition data from the output.
// This allows chaining multiple ignition edits.
func (in IgnitionInput) WithOutput(output *IgnitionOutput) IgnitionInput {
	if output == nil {
		return in
	}
	in.BootstrapIgnData = output.UpdatedBootstrapIgn
	in.MasterIgnData = output.UpdatedMasterIgn
	in.WorkerIgnData = output.UpdatedWorkerIgn
	return in
}

// IgnitionEditFunc is a function that edits ignition data.
type IgnitionEditFunc func(context.Context, IgnitionInput) (*IgnitionOutput, error)

// ApplyIgnitionEdits applies multiple ignition edit functions in sequence, passing the ignition output
// of each as input to the next. Returns the final output or the first error encountered.
func ApplyIgnitionEdits(ctx context.Context, in IgnitionInput, edits ...IgnitionEditFunc) (*IgnitionOutput, error) {
	output := &IgnitionOutput{
		UpdatedBootstrapIgn: in.BootstrapIgnData,
		UpdatedMasterIgn:    in.MasterIgnData,
		UpdatedWorkerIgn:    in.WorkerIgnData,
	}

	for _, edit := range edits {
		result, err := edit(ctx, in)
		if err != nil {
			return nil, err
		}
		output = result
		in = in.WithOutput(result)
	}

	return output, nil
}

// IgnitionOutput collects updated Ignition Data for Bootstrap, Master and Worker nodes.
type IgnitionOutput struct {
	UpdatedBootstrapIgn []byte
	UpdatedMasterIgn    []byte
	UpdatedWorkerIgn    []byte
}

// InfraReadyProvider defines the InfraReady hook, which is
// called after the initial infrastructure manifests have been created
// and InfrastructureReady == true on the cluster status, and before
// IgnitionProvider hook and creation of the control-plane machines.
type InfraReadyProvider interface {
	// InfraReady is called once cluster.Status.InfrastructureReady
	// is true, typically after load balancers have been provisioned. It can be used
	// to create DNS records.
	InfraReady(ctx context.Context, in InfraReadyInput) error
}

// InfraReadyInput collects the args passed to the InfraReady call.
type InfraReadyInput struct {
	// Client is the client for kube-apiserver running locally on the installer host.
	// It can be used to read the status of the cluster object on the local control plane.
	Client        client.Client
	InstallConfig *installconfig.InstallConfig
	InfraID       string
}

// PostProvider defines the PostProvision hook, which is called after
// machine provisioning has completed.
type PostProvider interface {
	PostProvision(ctx context.Context, in PostProvisionInput) error
}

// PostProvisionInput collects the args passed to the PostProvision hook.
type PostProvisionInput struct {
	Client        client.Client
	InstallConfig *installconfig.InstallConfig
	InfraID       string
}

// BootstrapDestroyer allows platform-specific behavior when
// destroying bootstrap resources.
type BootstrapDestroyer interface {
	DestroyBootstrap(ctx context.Context, in BootstrapDestroyInput) error
}

// BootstrapDestroyInput collects args passed to the DestroyBootstrap hook.
type BootstrapDestroyInput struct {
	Client   client.Client
	Metadata types.ClusterMetadata
}

// PostDestroyer allows platform-specific behavior after bootstrap has been destroyed and
// ClusterAPI has stopped running.
type PostDestroyer interface {
	PostDestroy(ctx context.Context, in PostDestroyerInput) error
}

// PostDestroyerInput collects args passed to the PostDestroyer hook.
type PostDestroyerInput struct {
	Metadata types.ClusterMetadata
}

// Timeouts allows platform provider to override the timeouts for certain phases.
type Timeouts interface {
	// When waiting for the network infrastructure to become ready.
	NetworkTimeout() time.Duration
	// When waiting for the machines to provision.
	ProvisionTimeout() time.Duration
}

// GatherEndpoint represents the valid values for connecting to the bootstrap nude
// in a public cluster to gather logs.
type GatherEndpoint string

const (
	// ExternalIP indicates that the machine status will include an ExternalIP that can be used for gather.
	ExternalIP GatherEndpoint = "ExternalIP"

	// InternalIP indicates that the machine status will only include InternalIPs.
	InternalIP GatherEndpoint = "InternalIP"

	// APILoadBalancer indicates that gather bootstrap should connect to the API load balancer.
	APILoadBalancer GatherEndpoint = "APILoadBalancer"
)
