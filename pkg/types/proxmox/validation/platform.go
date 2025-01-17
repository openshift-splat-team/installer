package validation

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/proxmox"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidatePlatform checks that the specified platform is valid.
func ValidatePlatform(p *proxmox.Platform, agentBasedInstallation bool, fldPath *field.Path, c *types.InstallConfig) field.ErrorList {

	allErrs := field.ErrorList{}

	// todo: proxmox validate platform

	return allErrs
}
