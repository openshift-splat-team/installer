package vsphere

import (
	"context"
	"fmt"

	"github.com/openshift/installer/pkg/types/vsphere"
	"github.com/sirupsen/logrus"
)

// PerComponentValidator validates per-component vSphere credentials for greenfield installations.
type PerComponentValidator struct {
	privilegeValidator *PrivilegeValidator
}

// NewPerComponentValidator creates a new PerComponentValidator instance.
func NewPerComponentValidator() *PerComponentValidator {
	return &PerComponentValidator{
		privilegeValidator: NewPrivilegeValidator(),
	}
}

// ValidatePerComponentCredentials validates all component credentials have required privileges.
// This is the integration point for greenfield installation flow (Story #6).
//
// Returns an error if any component's credentials are invalid or missing required privileges.
func (v *PerComponentValidator) ValidatePerComponentCredentials(ctx context.Context, platform *vsphere.Platform) error {
	logrus.Info("Validating per-component vSphere credentials")

	// If ComponentCredentials is not provided, skip per-component validation (legacy mode)
	if platform.ComponentCredentials == nil {
		logrus.Debug("No componentCredentials specified, skipping per-component validation")
		return nil
	}

	// Get default vCenter
	defaultVCenter := getDefaultVCenter(platform)

	// Validate installer credentials
	if err := v.validateComponent(ctx, "installer", platform.ComponentCredentials.Installer, defaultVCenter); err != nil {
		return fmt.Errorf("installer credentials validation failed: %w", err)
	}

	// Validate Machine API credentials
	if err := v.validateComponent(ctx, "machine-api", platform.ComponentCredentials.MachineAPI, defaultVCenter); err != nil {
		return fmt.Errorf("machine-api credentials validation failed: %w", err)
	}

	// Validate CSI Driver credentials
	if err := v.validateComponent(ctx, "csi-driver", platform.ComponentCredentials.CSIDriver, defaultVCenter); err != nil {
		return fmt.Errorf("csi-driver credentials validation failed: %w", err)
	}

	// Validate Cloud Controller credentials
	if err := v.validateComponent(ctx, "cloud-controller", platform.ComponentCredentials.CloudController, defaultVCenter); err != nil {
		return fmt.Errorf("cloud-controller credentials validation failed: %w", err)
	}

	// Validate Diagnostics credentials
	if err := v.validateComponent(ctx, "diagnostics", platform.ComponentCredentials.Diagnostics, defaultVCenter); err != nil {
		return fmt.Errorf("diagnostics credentials validation failed: %w", err)
	}

	logrus.Info("All component credentials validated successfully")
	return nil
}

// validateComponent validates a single component's credentials.
func (v *PerComponentValidator) validateComponent(ctx context.Context, component string, creds *vsphere.AccountCredentials, defaultVCenter string) error {
	if creds == nil {
		return fmt.Errorf("component %s credentials not provided", component)
	}

	// Determine vCenter to use (component-specific override or default)
	vCenter := defaultVCenter
	if creds.VCenter != "" {
		vCenter = creds.VCenter
	}

	logrus.Debugf("Validating %s credentials for vCenter %s", component, vCenter)

	// Validate credentials have required privileges
	result, err := v.privilegeValidator.ValidateComponentPrivileges(ctx, component, creds, vCenter)
	if err != nil {
		return fmt.Errorf("failed to validate privileges: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("component %s missing required privileges: %v on %s",
			component, result.MissingPrivileges, result.Scope)
	}

	logrus.Debugf("Component %s credentials validated successfully", component)
	return nil
}

// GetInstallerCredentials returns the credentials to use for infrastructure provisioning.
// Returns installer credentials if provided, otherwise falls back to legacy credentials.
func (v *PerComponentValidator) GetInstallerCredentials(platform *vsphere.Platform) (username, password, vcenter string) {
	defaultVCenter := getDefaultVCenter(platform)

	// Per-component mode: use installer credentials
	if platform.ComponentCredentials != nil && platform.ComponentCredentials.Installer != nil {
		creds := platform.ComponentCredentials.Installer
		vcenter = defaultVCenter
		if creds.VCenter != "" {
			vcenter = creds.VCenter
		}
		return creds.Username, creds.Password, vcenter
	}

	// Legacy mode: use platform credentials
	return getLegacyCredentials(platform), getLegacyPassword(platform), defaultVCenter
}

// IsPerComponentMode returns true if the platform is configured for per-component credentials.
func IsPerComponentMode(platform *vsphere.Platform) bool {
	return platform.ComponentCredentials != nil
}

// getDefaultVCenter returns the default vCenter server from the platform configuration.
func getDefaultVCenter(platform *vsphere.Platform) string {
	// Prefer VCenters[0].Server (new field)
	if len(platform.VCenters) > 0 {
		return platform.VCenters[0].Server
	}
	// Fall back to DeprecatedVCenter
	return platform.DeprecatedVCenter
}

// getLegacyCredentials returns the legacy username from platform configuration.
func getLegacyCredentials(platform *vsphere.Platform) string {
	// Prefer VCenters[0].Username (new field)
	if len(platform.VCenters) > 0 {
		return platform.VCenters[0].Username
	}
	// Fall back to DeprecatedUsername
	return platform.DeprecatedUsername
}

// getLegacyPassword returns the legacy password from platform configuration.
func getLegacyPassword(platform *vsphere.Platform) string {
	// Prefer VCenters[0].Password (new field)
	if len(platform.VCenters) > 0 {
		return platform.VCenters[0].Password
	}
	// Fall back to DeprecatedPassword
	return platform.DeprecatedPassword
}
