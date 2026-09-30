package external

// CloudControllerManager describes the type of cloud controller manager to be enabled.
type CloudControllerManager string

const (
	// CloudControllerManagerTypeExternal specifies that an external cloud provider is to be configured.
	CloudControllerManagerTypeExternal = "External"

	// CloudControllerManagerTypeNone specifies that no cloud provider is to be configured.
	CloudControllerManagerTypeNone = ""
)

// Platform stores configuration related to external cloud providers.
type Platform struct {
	// PlatformName holds the arbitrary string representing the infrastructure provider name, expected to be set at the installation time.
	// This field is solely for informational and reporting purposes and is not expected to be used for decision-making.
	// +kubebuilder:default:="Unknown"
	// +default="Unknown"
	// +kubebuilder:validation:XValidation:rule="oldSelf == 'Unknown' || self == oldSelf",message="platform name cannot be changed once set"
	// +optional
	PlatformName string `json:"platformName,omitempty"`

	// CloudControllerManager when set to external, this property will enable an external cloud provider.
	// +kubebuilder:default:=""
	// +default=""
	// +kubebuilder:validation:Enum="";External
	// +optional
	CloudControllerManager CloudControllerManager `json:"cloudControllerManager,omitempty"`

	// ClusterAPI configures a user-supplied Cluster API infrastructure provider
	// to provision the cluster's infrastructure. When it is unset, the installer
	// provisions nothing and the infrastructure is expected to exist already,
	// which is the behaviour of this platform before this field was introduced.
	//
	// This is a provisional interface for the External enablement pilot and is
	// not a supported installation API.
	// +optional
	ClusterAPI *ClusterAPIProvider `json:"clusterAPI,omitempty"`
}

// ProvisionsWithClusterAPI reports whether this platform asks the installer to
// provision infrastructure with a user-supplied Cluster API provider, rather
// than leaving it to be provisioned out of band.
//
// This is the one predicate that separates the two kinds of External install,
// and every place that has to tell them apart must call it: the platform's
// long-standing user-provisioned behaviour has to survive unchanged, and it
// only does so if the condition cannot drift between call sites.
//
// A nil receiver reports false, so callers holding an InstallConfig whose
// platform may be something else entirely can call it without a guard.
func (p *Platform) ProvisionsWithClusterAPI() bool {
	return p != nil && p.ClusterAPI != nil
}

// ClusterAPIProvider describes the Cluster API infrastructure provider that
// provisions the cluster's infrastructure. The installer starts the named
// controller against its temporary, local Cluster API control plane and
// applies the manifests the user supplies in the install directory; it has no
// knowledge of the provider's API and does not generate manifests for it.
type ClusterAPIProvider struct {
	// Name identifies the provider, for example "oci". It names the controller
	// in installer logs and in the developer-only artifact override
	// environment variables.
	Name string `json:"name"`

	// BinaryPath is a path on the machine running the installer to the provider
	// controller executable.
	//
	// Remote references -- a git repository, a container image, or a release
	// payload -- are not supported, and neither is digest or signature pinning.
	// Both are TBD after the pilot: the provider artifacts must already be
	// extracted onto this machine.
	BinaryPath string `json:"binaryPath"`

	// ComponentsPath is a path on the machine running the installer to the
	// provider's CRDs and component manifests, either a single YAML file or a
	// directory of them.
	//
	// Remote references and digest pinning are TBD after the pilot, as for
	// BinaryPath.
	ComponentsPath string `json:"componentsPath"`

	// Args are appended to the provider controller's command line, after the
	// arguments the installer supplies itself. Use it for flags a particular
	// provider requires and the installer does not know about.
	// +optional
	Args []string `json:"args,omitempty"`

	// Hooks names programs the installer runs at fixed points in the install.
	// +optional
	Hooks *Hooks `json:"hooks,omitempty"`
}

// Hooks names programs the installer runs at points in the flow where an
// integrated platform would run its own code.
//
// This is the seam that lets a partner automate the parts of provisioning
// that Cluster API does not cover, without the installer gaining any
// knowledge of their cloud. The motivating case is DNS. An integrated
// platform creates the cluster's `api` and `api-int` records itself once the
// load balancers exist -- for AWS that is InfraReady in
// pkg/infrastructure/aws/clusterapi/aws.go:112 -- but Cluster API has no
// contract for it: the core Cluster carries a single
// spec.controlPlaneEndpoint and there is no field for an internal endpoint at
// all. Without `api-int` the bootstrap node fails at its resolve-api-int-url
// stage and no control-plane machine can fetch its ignition, so something has
// to create those records, and on this platform it cannot be the installer.
//
// Each field is a path to an executable, relative to the External manifest
// directory in the install directory. It must stay inside that directory: the
// install directory is the unit a user copies, archives and hands to someone
// else, and a hook reaching outside it would run something that did not
// travel with it.
//
// Every hook is optional. Leaving one unset is a supported configuration --
// the records may be created out of band, which is what the existing
// non-Cluster-API External CI does with CloudFormation -- so an absent hook
// is a warning naming the consequence, never an error.
type Hooks struct {
	// InfraReady names a program run once the Cluster reports
	// status.infrastructureReady, after the provider has created the network
	// and load balancers and before any machine is created. It is the point
	// at which the load balancer addresses first exist and the last point at
	// which DNS can be created in time for the bootstrap node to use it.
	// +optional
	InfraReady string `json:"infraReady,omitempty"`

	// PreDestroy names a program run by `destroy cluster` after the local
	// control plane has been restored and before the Cluster is deleted.
	//
	// It is the counterpart of InfraReady and exists because that hook's
	// resources are invisible to Cluster API: deleting the Cluster removes
	// what the provider built and nothing else. Running before the delete
	// rather than after is deliberate -- a DNS alias record is described in
	// terms of the load balancer it points at, so it is cheaper to remove
	// while that load balancer still exists.
	// +optional
	PreDestroy string `json:"preDestroy,omitempty"`
}
