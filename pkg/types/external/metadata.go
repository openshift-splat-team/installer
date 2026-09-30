package external

// Metadata contains External metadata (e.g. for uninstalling the cluster).
//
// Its presence is what identifies the platform: ClusterPlatformMetadata.Platform
// reports "external" only when this member is set, and pkg/clusterapi's
// System.Run refuses to start without a platform name. So the External CAPI
// path needs this written even though the installer has nothing
// platform-specific to record.
//
// PlatformName is informational, mirroring the install-config field of the
// same name. The Cluster API provider artifact paths are deliberately not
// recorded here: they are machine-local paths, metadata.json travels with
// support bundles, and whether `destroy cluster` needs them is an open
// question in the enhancement rather than a settled design.
type Metadata struct {
	// PlatformName holds the arbitrary string representing the infrastructure
	// provider name that was set at installation time.
	PlatformName string `json:"platformName,omitempty"`
}
