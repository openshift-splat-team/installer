package proxmox

// MachinePool stores the configuration for a machine pool installed
// on vSphere.
type MachinePool struct {
	// +optional
	NumCores int32 `json:"numCores"`

	// +optional
	NumSockets int32 `json:"numSockets"`

	// Memory is the size of a VM's memory in MB.
	//
	// +optional
	MemoryMiB int64 `json:"memoryMiB"`

	// OSDisk defines the storage for instance.
	//
	// +optional
	OSDisk `json:"osDisk"`

	// Zones defines available zones
	// Zones is available in TechPreview.
	//
	// +omitempty
	// Zones []string `json:"zones,omitempty"`
}

// OSDisk defines the disk for a virtual machine.
type OSDisk struct {
	// DiskSizeGB defines the size of disk in GB.
	//
	// +optional
	DiskSizeGB int32 `json:"diskSizeGB"`
}

// Set sets the values from `required` to `p`.
func (p *MachinePool) Set(required *MachinePool) {
	if required == nil || p == nil {
		return
	}

	if required.NumCores != 0 {
		p.NumCores = required.NumCores
	}

	if required.NumSockets != 0 {
		p.NumSockets = required.NumSockets
	}

	if required.MemoryMiB != 0 {
		p.MemoryMiB = required.MemoryMiB
	}

	if required.OSDisk.DiskSizeGB != 0 {
		p.OSDisk.DiskSizeGB = required.OSDisk.DiskSizeGB
	}

	/*
		if len(required.Zones) > 0 {
			p.Zones = required.Zones
		}

	*/
}
