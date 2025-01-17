package proxmox

import (
	"errors"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/proxmox/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(ic *types.InstallConfig) error {

	// todo: proxmox add validation

	if ic.Platform.Proxmox == nil {
		return errors.New(field.Required(field.NewPath("platform", "proxmox"), "proxmox validation requires a proxmox platform configuration").Error())
	}
	return validation.ValidatePlatform(ic.Platform.Proxmox, false, field.NewPath("platform").Child("proxmox"), ic).ToAggregate()

}
func ValidateForProvisioning(ic *types.InstallConfig) error {
	allErrs := field.ErrorList{}

	// todo: proxmox add validation for provisioning

	return allErrs.ToAggregate()
}
