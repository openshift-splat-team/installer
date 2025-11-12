package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types/proxmox"
)

// ValidateMachinePool checks that the specified machine pool is valid.
func ValidateMachinePool(platform *proxmox.Platform, p *proxmox.MachinePool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if p.NumCores < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("numCores"), p.NumCores, "number of cores must be positive"))
	}

	if p.NumCores == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("numCores"), "must specify the number of cores"))
	}

	if p.NumSockets < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("numSockets"), p.NumSockets, "number of sockets must be positive"))
	}

	if p.NumSockets == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("numSockets"), "must specify the number of sockets"))
	}

	if p.MemoryMiB < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("memoryMiB"), p.MemoryMiB, "memory size must be positive"))
	}

	if p.MemoryMiB == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("memoryMiB"), "must specify the memory size"))
	}

	// Minimum memory requirement check (at least 8GB for OCP nodes)
	const minMemoryMiB = 8192
	if p.MemoryMiB > 0 && p.MemoryMiB < minMemoryMiB {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("memoryMiB"), p.MemoryMiB,
			fmt.Sprintf("memory size must be at least %d MiB (8 GB)", minMemoryMiB)))
	}

	if p.OSDisk.DiskSizeGB < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("osDisk", "diskSizeGB"), p.OSDisk.DiskSizeGB, "disk size must be positive"))
	}

	if p.OSDisk.DiskSizeGB == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("osDisk", "diskSizeGB"), "must specify the OS disk size"))
	}

	// Minimum disk size requirement (at least 120GB for OCP)
	const minDiskSizeGB = 120
	if p.OSDisk.DiskSizeGB > 0 && p.OSDisk.DiskSizeGB < minDiskSizeGB {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("osDisk", "diskSizeGB"), p.OSDisk.DiskSizeGB,
			fmt.Sprintf("disk size must be at least %d GB", minDiskSizeGB)))
	}

	return allErrs
}
