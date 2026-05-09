package vsphere

// Credential holds a vCenter username and password for a single component.
type Credential struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// ComponentCredentials holds per-component vCenter credentials that allow
// privilege separation between provisioning and day-2 operations.
//
// All fields are optional. When omitted the installer falls back to the
// shared vCenter credentials supplied at the VCenter level.
type ComponentCredentials struct {
	// MachineAPI credentials for VM lifecycle management.
	// +optional
	MachineAPI *Credential `json:"machineAPI,omitempty"`

	// CSIDriver credentials for persistent storage operations.
	// +optional
	CSIDriver *Credential `json:"csiDriver,omitempty"`

	// CloudController credentials for read-only node discovery.
	// +optional
	CloudController *Credential `json:"cloudController,omitempty"`

	// Diagnostics (vSphere Problem Detector) credentials for monitoring.
	// +optional
	Diagnostics *Credential `json:"diagnostics,omitempty"`
}
