package vsphere

import "fmt"

// ComponentCredentials represents per-component vCenter credentials for
// multi-account credential management (Story #17).
//
// This enables privilege separation between provisioning (high privilege) and
// day-2 operations (restricted privilege), reducing blast radius and improving
// compliance with SOC2, PCI-DSS requirements.
type ComponentCredentials struct {
	// Installer credentials for infrastructure provisioning (~50 privileges)
	Installer *CredentialRef `json:"installer,omitempty"`

	// MachineAPI credentials for VM lifecycle management (35 privileges)
	MachineAPI *CredentialRef `json:"machineAPI,omitempty"`

	// Storage credentials for persistent storage operations (10-15 privileges)
	Storage *CredentialRef `json:"storage,omitempty"`

	// CloudController credentials for read-only node discovery (~10 privileges)
	CloudController *CredentialRef `json:"cloudController,omitempty"`

	// Diagnostics credentials for configuration validation (~16 privileges)
	Diagnostics *CredentialRef `json:"diagnostics,omitempty"`
}

// CredentialRef references credentials for a component, supporting both
// inline credentials and external credential files.
type CredentialRef struct {
	// Username for vCenter authentication
	Username string `json:"username,omitempty"`

	// Password for vCenter authentication
	Password string `json:"password,omitempty"`

	// SecretRef references a Kubernetes secret (for runtime credential distribution)
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// SecretReference identifies a Kubernetes secret containing vCenter credentials.
type SecretReference struct {
	// Name of the secret
	Name string `json:"name"`

	// Namespace containing the secret
	Namespace string `json:"namespace"`
}

// ParseComponentCredentials extracts component-specific credentials from install-config.yaml.
//
// For multi-vCenter deployments, credentials are keyed by vCenter FQDN:
//
//	vcenter1.example.com.username: "machine-api@vsphere.local"
//	vcenter1.example.com.password: "password1"
//
// This is a simplified implementation that validates the ComponentCredentials structure.
// Real install-config parsing would use the types.InstallConfig struct and extract from
// platform.vsphere.componentCredentials.
func ParseComponentCredentials(installConfig interface{}) (*ComponentCredentials, error) {
	// In a real implementation, this would:
	// 1. Cast installConfig to *types.InstallConfig
	// 2. Extract platform.vsphere.componentCredentials
	// 3. Parse each component's credentials
	// 4. Validate username/password are present
	// 5. Handle multi-vCenter credential format
	//
	// For this implementation, we accept a pre-structured ComponentCredentials
	// and validate its format.

	if installConfig == nil {
		return nil, fmt.Errorf("install-config is nil")
	}

	// Type assertion to ComponentCredentials
	creds, ok := installConfig.(*ComponentCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid install-config type, expected *ComponentCredentials")
	}

	// Validate each component's credentials
	if err := validateCredentialRef("installer", creds.Installer); err != nil {
		return nil, err
	}
	if err := validateCredentialRef("machineAPI", creds.MachineAPI); err != nil {
		return nil, err
	}
	if err := validateCredentialRef("storage", creds.Storage); err != nil {
		return nil, err
	}
	if err := validateCredentialRef("cloudController", creds.CloudController); err != nil {
		return nil, err
	}
	if err := validateCredentialRef("diagnostics", creds.Diagnostics); err != nil {
		return nil, err
	}

	return creds, nil
}

// validateCredentialRef validates that a credential ref has username and password.
func validateCredentialRef(component string, cred *CredentialRef) error {
	if cred == nil {
		return fmt.Errorf("%s: credentials not provided", component)
	}
	if cred.Username == "" {
		return fmt.Errorf("%s: username is empty", component)
	}
	if cred.Password == "" && cred.SecretRef == nil {
		return fmt.Errorf("%s: password is empty and no secretRef provided", component)
	}
	return nil
}

// GetCredentialsForVCenter retrieves credentials for a specific vCenter from
// a component's credential set.
//
// For single-vCenter deployments, returns the single credential.
// For multi-vCenter deployments, looks up by vCenter FQDN.
func GetCredentialsForVCenter(vcenterFQDN string, cred *CredentialRef) (username, password string, err error) {
	if cred == nil {
		return "", "", fmt.Errorf("credential ref is nil")
	}

	// For single-vCenter deployments, credentials are directly in the CredentialRef
	if cred.Username != "" {
		return cred.Username, cred.Password, nil
	}

	// For multi-vCenter deployments with secretRef, credentials are stored
	// in a Kubernetes secret with FQDN-based keys (handled by component operators)
	if cred.SecretRef != nil {
		// Return placeholder - component operators will resolve from secret
		return fmt.Sprintf("%s.username", vcenterFQDN), "", nil
	}

	return "", "", fmt.Errorf("no credentials found for vCenter %s", vcenterFQDN)
}
