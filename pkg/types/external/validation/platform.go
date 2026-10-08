// Package validation validates the External platform section of the
// install-config.
package validation

import (
	"fmt"
	"path/filepath"

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
	allErrs = append(allErrs, validateHooks(p.Hooks, fldPath.Child("hooks"))...)
	return allErrs
}

// validateHooks checks the shape of each configured hook.
//
// Only containment is decided here, not existence or executability: those
// need the install directory, which this package does not have, and they are
// checked where the hook is about to run. Containment can be decided from the
// string alone, and has to be, because it is the property that makes the
// install directory a self-contained unit.
//
// Args are deliberately not validated at all. They are the partner's
// contract with their own program, and the installer does not know what a
// valid one looks like -- inventing a rule here would reject a working
// configuration for a program this code has never seen.
func validateHooks(h *external.Hooks, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if h == nil {
		return allErrs
	}
	for _, hook := range []struct {
		name string
		hook *external.Hook
	}{
		{"infraReady", h.InfraReady},
		{"postProvision", h.PostProvision},
		{"preDestroy", h.PreDestroy},
	} {
		allErrs = append(allErrs, validateHook(hook.hook, fldPath.Child(hook.name))...)
	}
	return allErrs
}

// validateHook requires a program, given as a relative path that stays inside
// the External manifest directory.
//
// filepath.Localize is the containment check rather than a hand-written scan
// for "..": it rejects absolute paths, parent traversal and platform-specific
// escapes in one place, and it is the same function the standard library uses
// to decide whether a path from an untrusted source can be joined to a root.
func validateHook(h *external.Hook, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if h == nil {
		return allErrs
	}
	// An empty program with arguments set is the mistake worth catching: it
	// is a hook the user believes they configured and that would silently
	// never run.
	if h.Program == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("program"),
			"path to the hook program is required when the hook is configured"))
		return allErrs
	}
	if _, err := filepath.Localize(h.Program); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("program"), h.Program,
			fmt.Sprintf("must be a relative path inside the %q directory of the install directory, "+
				"so that the hook travels with the install directory it belongs to", external.ManifestDir)))
	}
	return allErrs
}
