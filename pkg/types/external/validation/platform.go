// Package validation validates the External platform section of the
// install-config.
package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types/external"
)

// ValidatePlatform checks that the specified platform is valid.
func ValidatePlatform(p *external.Platform, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if p == nil {
		return allErrs
	}
	allErrs = append(allErrs, validateClusterAPI(p.ClusterAPI, fldPath.Child("clusterAPI"))...)
	return allErrs
}

// validateClusterAPI checks the user-supplied Cluster API infrastructure
// provider. The three paths are required together: a provider with no binary
// cannot be started, and one with no components has no CRDs, so the manifests
// the user supplies in the install directory would be rejected by the local
// control plane.
//
// Only the shape of the configuration is checked here. Whether the paths
// resolve to a usable executable and to installable manifests is decided when
// the controller starts, by the resolver in pkg/clusterapi: that check needs
// the filesystem, and duplicating it here would mean two implementations
// disagreeing about what is valid.
func validateClusterAPI(p *external.ClusterAPIProvider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if p == nil {
		return allErrs
	}
	if p.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "provider name is required"))
	}
	if p.BinaryPath == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("binaryPath"),
			"path to the provider controller binary is required"))
	}
	if p.ComponentsPath == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("componentsPath"),
			"path to the provider components is required"))
	}
	return allErrs
}
