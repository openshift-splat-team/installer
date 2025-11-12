package defaults

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/proxmox"
)

// SetPlatformDefaults sets the defaults for the platform.
func SetPlatformDefaults(p *proxmox.Platform, installConfig *types.InstallConfig) {
	// Set default port if not specified
	for i := range p.Proxmoxs {
		if p.Proxmoxs[i].Port == 0 {
			p.Proxmoxs[i].Port = 8006 // Default Proxmox API port
		}
	}

	// Set defaults for the default machine pool if specified
	if p.DefaultMachinePlatform != nil {
		SetMachinePoolDefaults(p.DefaultMachinePlatform)
	}

	// Set defaults for control plane machine pool
	if installConfig.ControlPlane != nil && installConfig.ControlPlane.Platform.Proxmox != nil {
		SetMachinePoolDefaults(installConfig.ControlPlane.Platform.Proxmox)
	}

	// Set defaults for compute machine pools
	for i := range installConfig.Compute {
		if installConfig.Compute[i].Platform.Proxmox != nil {
			SetMachinePoolDefaults(installConfig.Compute[i].Platform.Proxmox)
		}
	}
}

// SetMachinePoolDefaults sets the defaults for a machine pool.
func SetMachinePoolDefaults(pool *proxmox.MachinePool) {
	if pool.NumCores == 0 {
		pool.NumCores = 4
	}

	if pool.NumSockets == 0 {
		pool.NumSockets = 1
	}

	if pool.MemoryMiB == 0 {
		pool.MemoryMiB = 16384 // 16GB
	}

	if pool.OSDisk.DiskSizeGB == 0 {
		pool.OSDisk.DiskSizeGB = 120
	}
}
