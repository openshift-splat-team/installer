package vsphere

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// CredentialsFileDefaultPath is the default location for the vSphere credentials file.
var CredentialsFileDefaultPath = filepath.Join(os.Getenv("HOME"), ".vsphere", "credentials")

// CredentialsFile represents the YAML structure of the vSphere credentials file.
// The file uses vCenter FQDNs as top-level keys, with component-specific credentials nested underneath.
//
// Example YAML format:
//
//	vcenter1.example.com:
//	  installer:
//	    username: admin@vsphere.local
//	    password: <password>
//	  machine-api:
//	    username: ocp-machine-api@vsphere.local
//	    password: <password>
//	  csi-driver:
//	    username: ocp-csi@vsphere.local
//	    password: <password>
//	  cloud-controller:
//	    username: ocp-ccm@vsphere.local
//	    password: <password>
//	  diagnostics:
//	    username: ocp-diagnostics@vsphere.local
//	    password: <password>
type CredentialsFile map[string]VCenterComponentCredentials

// VCenterComponentCredentials holds component credentials for a single vCenter.
type VCenterComponentCredentials struct {
	Installer       *ComponentAccount `yaml:"installer,omitempty"`
	MachineAPI      *ComponentAccount `yaml:"machine-api,omitempty"`
	CSIDriver       *ComponentAccount `yaml:"csi-driver,omitempty"`
	CloudController *ComponentAccount `yaml:"cloud-controller,omitempty"`
	Diagnostics     *ComponentAccount `yaml:"diagnostics,omitempty"`
}

// ComponentAccount holds username and password for a component.
type ComponentAccount struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoadCredentialsFile reads and parses the vSphere credentials file from the specified path.
// Returns the parsed credentials file or an error if the file cannot be read or parsed.
// If the file does not exist, returns (nil, nil) to indicate graceful fallback.
func LoadCredentialsFile(path string) (*CredentialsFile, error) {
	// If path is empty, use default
	if path == "" {
		path = CredentialsFileDefaultPath
	}

	// Check if file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// File doesn't exist - graceful fallback (not an error)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat credentials file %s: %w", path, err)
	}

	// Validate file permissions (must be 0600)
	if err := validateFilePermissions(path, info); err != nil {
		return nil, err
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file %s: %w", path, err)
	}

	// Handle empty file (graceful fallback)
	if len(data) == 0 {
		return nil, nil
	}

	// Parse YAML
	var credsFile CredentialsFile
	if err := yaml.Unmarshal(data, &credsFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML credentials file %s: %w", path, err)
	}

	return &credsFile, nil
}

// validateFilePermissions ensures the credentials file has 0600 permissions (read/write for owner only).
func validateFilePermissions(path string, info os.FileInfo) error {
	mode := info.Mode()
	perm := mode.Perm()

	// File must be 0600 (read/write for owner only)
	if perm != 0600 {
		return fmt.Errorf("credentials file %s has permissions %04o, must be 0600", path, perm)
	}

	return nil
}

// MergeWithComponentCredentials merges credentials from the credentials file into the install-config
// ComponentCredentials. Precedence: install-config > credentials file.
// For each component:
// - If install-config has credentials for that component, use install-config (ignore file)
// - If install-config does NOT have credentials, use credentials file (if available)
// - If neither has credentials, component will fall back to legacy passthrough mode
func MergeWithComponentCredentials(
	installConfigCreds *vsphere.ComponentCredentials,
	credsFile *CredentialsFile,
	vCenterFQDN string,
) *vsphere.ComponentCredentials {
	// If no credentials file, return install-config credentials as-is
	if credsFile == nil {
		return installConfigCreds
	}

	// Get vCenter-specific credentials from file
	vcenterCreds, exists := (*credsFile)[vCenterFQDN]
	if !exists {
		// No credentials for this vCenter in file, return install-config as-is
		return installConfigCreds
	}

	// If no install-config component credentials exist, initialize empty struct
	if installConfigCreds == nil {
		installConfigCreds = &vsphere.ComponentCredentials{}
	}

	// Merge each component (install-config takes precedence)
	if installConfigCreds.Installer == nil && vcenterCreds.Installer != nil {
		installConfigCreds.Installer = &vsphere.AccountCredentials{
			Username: vcenterCreds.Installer.Username,
			Password: vcenterCreds.Installer.Password,
			VCenter:  vCenterFQDN,
		}
	}

	if installConfigCreds.MachineAPI == nil && vcenterCreds.MachineAPI != nil {
		installConfigCreds.MachineAPI = &vsphere.AccountCredentials{
			Username: vcenterCreds.MachineAPI.Username,
			Password: vcenterCreds.MachineAPI.Password,
			VCenter:  vCenterFQDN,
		}
	}

	if installConfigCreds.CSIDriver == nil && vcenterCreds.CSIDriver != nil {
		installConfigCreds.CSIDriver = &vsphere.AccountCredentials{
			Username: vcenterCreds.CSIDriver.Username,
			Password: vcenterCreds.CSIDriver.Password,
			VCenter:  vCenterFQDN,
		}
	}

	if installConfigCreds.CloudController == nil && vcenterCreds.CloudController != nil {
		installConfigCreds.CloudController = &vsphere.AccountCredentials{
			Username: vcenterCreds.CloudController.Username,
			Password: vcenterCreds.CloudController.Password,
			VCenter:  vCenterFQDN,
		}
	}

	if installConfigCreds.Diagnostics == nil && vcenterCreds.Diagnostics != nil {
		installConfigCreds.Diagnostics = &vsphere.AccountCredentials{
			Username: vcenterCreds.Diagnostics.Username,
			Password: vcenterCreds.Diagnostics.Password,
			VCenter:  vCenterFQDN,
		}
	}

	return installConfigCreds
}
