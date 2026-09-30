// Package external contains generic structures for installer
// configuration and management.
// +k8s:deepcopy-gen=package
package external

// Name is name for the External platform.
const Name string = "external"

// ManifestDir is the directory, relative to the install directory, holding the
// Cluster API manifests the user supplies for the External platform: the
// Cluster and the infrastructure object its provider reconciles, with Machine
// objects in the "machines" subdirectory.
//
// It is deliberately not the "cluster-api" directory the installer uses for
// the platforms it generates manifests for, for two reasons. That directory is
// an asset load path, and the asset store discards assets whose dependencies
// are dirty, which silently throws away files placed there alongside an
// unconsumed install-config.yaml. It is also where the installer unpacks the
// Cluster API and envtest binaries at provisioning time, so it is installer
// scratch space rather than a user interface.
//
// The name is provisional and may change before this lands.
const ManifestDir string = "external-install"

// MachineManifestDir is the subdirectory of ManifestDir holding Machine
// objects. They are separated because the installer creates them in a second
// stage, after the infrastructure reports ready.
const MachineManifestDir string = "machines"
