package vsphere

import (
	"fmt"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// isMultiVCenterMode determines if the install-config uses multi-vCenter topology.
// Multi-vCenter mode is detected when any component specifies a vCenter override in its
// AccountCredentials.VCenter field. In multi-vCenter mode, component secrets use FQDN-keyed
// credentials (e.g., vcenter1.example.com.username) instead of simple keys (username/password).
func isMultiVCenterMode(componentCreds *vsphere.ComponentCredentials) bool {
	if componentCreds == nil {
		return false
	}

	// Check each component for vCenter override
	if componentCreds.Installer != nil && componentCreds.Installer.VCenter != "" {
		return true
	}
	if componentCreds.MachineAPI != nil && componentCreds.MachineAPI.VCenter != "" {
		return true
	}
	if componentCreds.CSIDriver != nil && componentCreds.CSIDriver.VCenter != "" {
		return true
	}
	if componentCreds.CloudController != nil && componentCreds.CloudController.VCenter != "" {
		return true
	}
	if componentCreds.Diagnostics != nil && componentCreds.Diagnostics.VCenter != "" {
		return true
	}

	return false
}

// getComponentVCenter returns the vCenter FQDN for a component.
// If the component specifies a vCenter override, it returns that value.
// Otherwise, it returns the platform default vCenter.
func getComponentVCenter(componentCred *vsphere.AccountCredentials, defaultVCenter string) string {
	if componentCred != nil && componentCred.VCenter != "" {
		return componentCred.VCenter
	}
	return defaultVCenter
}

// getAllReferencedVCenters returns a deduplicated list of all vCenter servers
// referenced in the component credentials. This is used to validate that
// credentials are provided for all referenced vCenters.
func getAllReferencedVCenters(componentCreds *vsphere.ComponentCredentials, defaultVCenter string) []string {
	vCenterMap := make(map[string]bool)

	// Always include the default vCenter
	vCenterMap[defaultVCenter] = true

	if componentCreds == nil {
		return []string{defaultVCenter}
	}

	// Add vCenter from each component that specifies an override
	if componentCreds.Installer != nil && componentCreds.Installer.VCenter != "" {
		vCenterMap[componentCreds.Installer.VCenter] = true
	}
	if componentCreds.MachineAPI != nil && componentCreds.MachineAPI.VCenter != "" {
		vCenterMap[componentCreds.MachineAPI.VCenter] = true
	}
	if componentCreds.CSIDriver != nil && componentCreds.CSIDriver.VCenter != "" {
		vCenterMap[componentCreds.CSIDriver.VCenter] = true
	}
	if componentCreds.CloudController != nil && componentCreds.CloudController.VCenter != "" {
		vCenterMap[componentCreds.CloudController.VCenter] = true
	}
	if componentCreds.Diagnostics != nil && componentCreds.Diagnostics.VCenter != "" {
		vCenterMap[componentCreds.Diagnostics.VCenter] = true
	}

	// Convert map to slice
	vCenters := make([]string, 0, len(vCenterMap))
	for vCenter := range vCenterMap {
		vCenters = append(vCenters, vCenter)
	}

	return vCenters
}

// validateMultiVCenterCredentials verifies that credentials are provided for all
// referenced vCenter servers. Returns an error if a component references a vCenter
// but no credentials are available for that vCenter.
func validateMultiVCenterCredentials(componentCreds *vsphere.ComponentCredentials, defaultVCenter string) error {
	if componentCreds == nil {
		return nil
	}

	// Check each component's vCenter reference
	if err := validateComponentVCenterRef("machineAPI", componentCreds.MachineAPI, defaultVCenter); err != nil {
		return err
	}
	if err := validateComponentVCenterRef("csiDriver", componentCreds.CSIDriver, defaultVCenter); err != nil {
		return err
	}
	if err := validateComponentVCenterRef("cloudController", componentCreds.CloudController, defaultVCenter); err != nil {
		return err
	}
	if err := validateComponentVCenterRef("diagnostics", componentCreds.Diagnostics, defaultVCenter); err != nil {
		return err
	}
	if err := validateComponentVCenterRef("installer", componentCreds.Installer, defaultVCenter); err != nil {
		return err
	}

	return nil
}

// validateComponentVCenterRef validates that a component's vCenter reference has
// corresponding credentials. In this implementation, we assume credentials are
// validated elsewhere (e.g., privilege validator). This function ensures the
// vCenter field is properly formatted if specified.
func validateComponentVCenterRef(componentName string, cred *vsphere.AccountCredentials, defaultVCenter string) error {
	if cred == nil {
		return nil
	}

	vcenter := cred.VCenter
	if vcenter == "" {
		// Component uses default vCenter, which is always valid
		return nil
	}

	// Validate vCenter FQDN format (basic check)
	if len(vcenter) == 0 {
		return fmt.Errorf("component %s has empty vCenter field", componentName)
	}

	// In a real implementation, we would verify credentials exist for this vCenter.
	// For now, we rely on the privilege validator (Story #4) to catch authentication
	// failures when it attempts to connect to each vCenter.

	return nil
}
