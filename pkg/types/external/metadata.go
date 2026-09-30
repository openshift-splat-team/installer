package external

// Metadata contains External metadata (e.g. for uninstalling the cluster).
//
// Its presence is what identifies the platform: ClusterPlatformMetadata.Platform
// reports "external" only when this member is set, and pkg/clusterapi's
// System.Run refuses to start without a platform name. So the External CAPI
// path needs this written even though the installer has nothing
// platform-specific to record.
type Metadata struct {
	// PlatformName holds the arbitrary string representing the infrastructure
	// provider name that was set at installation time. It is informational,
	// mirroring the install-config field of the same name.
	PlatformName string `json:"platformName,omitempty"`

	// ClusterAPI describes the user-supplied Cluster API infrastructure
	// provider that provisioned this cluster. It is nil for an External
	// install that provisioned nothing.
	// +optional
	ClusterAPI *ClusterAPIMetadata `json:"clusterAPI,omitempty"`
}

// ClusterAPIMetadata records where the user-supplied Cluster API
// infrastructure provider came from, so that `destroy cluster` can start the
// same provider again.
//
// Destroy cannot work without this. The installer has no SDK for the
// partner's cloud, so it cannot sweep resources by tag the way the integrated
// destroyers do; the only way to remove the infrastructure is to hand the
// objects back to the provider that created them and let it delete them. That
// requires running the provider, and by the time `destroy cluster` runs the
// install-config that named it has been consumed.
//
// Only the artifact paths and a digest are recorded. metadata.json travels
// with support bundles, so this deliberately holds nothing but locations:
// never a credential, and never the contents of a file. The paths are
// machine-local, which means destroy has to run somewhere the artifacts are
// still reachable -- a real limitation, stated here rather than discovered.
type ClusterAPIMetadata struct {
	// Name identifies the provider. It names the controller in logs and
	// selects the developer-only artifact override environment variables.
	Name string `json:"name"`

	// BinaryPath is the absolute path to the provider controller executable.
	BinaryPath string `json:"binaryPath"`

	// No digest is recorded. metadata.json is written before the provider
	// artifacts are resolved and hashed, so any digest here would be either
	// absent or a second, differently-timed answer to a question the install
	// log already answers: both SetExternalProvider and the destroyer log the
	// binary's SHA-256, and comparing those two lines is what identifies a
	// destroy run against a different build than the install.

	// ComponentsPath is the absolute path to the provider's CRDs and
	// component manifests, either a file or a directory.
	ComponentsPath string `json:"componentsPath"`

	// Args are the extra arguments that were appended to the controller
	// command line, repeated here so that destroy starts the provider the
	// same way the install did.
	// +optional
	Args []string `json:"args,omitempty"`
}
