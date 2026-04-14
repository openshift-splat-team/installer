package vsphere

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// TestCredentialsFileReading_SingleVCenter tests reading YAML credentials file with single vCenter
func TestCredentialsFileReading_SingleVCenter(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create test credentials file with single vCenter
	yamlContent := `vcenter1.example.com:
  installer:
    username: installer@vsphere.local
    password: installer-password
  machine-api:
    username: machine-api@vsphere.local
    password: machine-api-password
  csi-driver:
    username: csi-driver@vsphere.local
    password: csi-password
  cloud-controller:
    username: cloud-controller@vsphere.local
    password: ccm-password
  diagnostics:
    username: diagnostics@vsphere.local
    password: diagnostics-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0600)
	if !assert.NoError(t, err, "Failed to create test credentials file") {
		return
	}

	// Parse credentials file
	credsFile, err := LoadCredentialsFile(credFilePath)
	if !assert.NoError(t, err, "Failed to load credentials file") {
		return
	}
	if !assert.NotNil(t, credsFile, "Credentials file should not be nil") {
		return
	}

	// Verify vCenter exists in file
	vcenterCreds, exists := (*credsFile)["vcenter1.example.com"]
	if !assert.True(t, exists, "vCenter vcenter1.example.com should exist in credentials file") {
		return
	}

	// Verify all 5 component accounts detected
	assert.NotNil(t, vcenterCreds.Installer, "Installer credentials should be present")
	assert.Equal(t, "installer@vsphere.local", vcenterCreds.Installer.Username)
	assert.Equal(t, "installer-password", vcenterCreds.Installer.Password)

	assert.NotNil(t, vcenterCreds.MachineAPI, "MachineAPI credentials should be present")
	assert.Equal(t, "machine-api@vsphere.local", vcenterCreds.MachineAPI.Username)
	assert.Equal(t, "machine-api-password", vcenterCreds.MachineAPI.Password)

	assert.NotNil(t, vcenterCreds.CSIDriver, "CSIDriver credentials should be present")
	assert.Equal(t, "csi-driver@vsphere.local", vcenterCreds.CSIDriver.Username)
	assert.Equal(t, "csi-password", vcenterCreds.CSIDriver.Password)

	assert.NotNil(t, vcenterCreds.CloudController, "CloudController credentials should be present")
	assert.Equal(t, "cloud-controller@vsphere.local", vcenterCreds.CloudController.Username)
	assert.Equal(t, "ccm-password", vcenterCreds.CloudController.Password)

	assert.NotNil(t, vcenterCreds.Diagnostics, "Diagnostics credentials should be present")
	assert.Equal(t, "diagnostics@vsphere.local", vcenterCreds.Diagnostics.Username)
	assert.Equal(t, "diagnostics-password", vcenterCreds.Diagnostics.Password)
}

// TestCredentialsFileReading_MultiVCenter tests reading YAML credentials file with multiple vCenters
func TestCredentialsFileReading_MultiVCenter(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create test credentials file with 2 vCenters
	yamlContent := `vcenter1.example.com:
  installer:
    username: vc1-installer@vsphere.local
    password: vc1-installer-password
  machine-api:
    username: vc1-machine-api@vsphere.local
    password: vc1-machine-api-password

vcenter2.example.com:
  installer:
    username: vc2-installer@vsphere.local
    password: vc2-installer-password
  csi-driver:
    username: vc2-csi-driver@vsphere.local
    password: vc2-csi-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Parse credentials file
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err, "Failed to load credentials file")
	assert.NotNil(t, credsFile, "Credentials file should not be nil")

	// Verify both vCenters exist
	vc1Creds, exists := (*credsFile)["vcenter1.example.com"]
	assert.True(t, exists, "vCenter vcenter1.example.com should exist")
	vc2Creds, exists := (*credsFile)["vcenter2.example.com"]
	assert.True(t, exists, "vCenter vcenter2.example.com should exist")

	// Verify vCenter1 credentials
	assert.NotNil(t, vc1Creds.Installer)
	assert.Equal(t, "vc1-installer@vsphere.local", vc1Creds.Installer.Username)
	assert.NotNil(t, vc1Creds.MachineAPI)
	assert.Equal(t, "vc1-machine-api@vsphere.local", vc1Creds.MachineAPI.Username)

	// Verify vCenter2 credentials
	assert.NotNil(t, vc2Creds.Installer)
	assert.Equal(t, "vc2-installer@vsphere.local", vc2Creds.Installer.Username)
	assert.NotNil(t, vc2Creds.CSIDriver)
	assert.Equal(t, "vc2-csi-driver@vsphere.local", vc2Creds.CSIDriver.Username)
}

// TestCredentialsFilePermissions_Reject0644 tests rejection of file with 0644 permissions
func TestCredentialsFilePermissions_Reject0644(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create test credentials file
	yamlContent := `vcenter1.example.com:
  installer:
    username: test@vsphere.local
    password: test-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0644)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Attempt to read credentials file (should fail due to permissions)
	_, err = LoadCredentialsFile(credFilePath)
	assert.Error(t, err, "Should fail with permissions error")
	assert.Contains(t, err.Error(), "has permissions 0644")
	assert.Contains(t, err.Error(), "must be 0600")
}

// TestCredentialsFilePermissions_Reject0777 tests rejection of file with 0777 permissions
func TestCredentialsFilePermissions_Reject0777(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create test credentials file
	yamlContent := `vcenter1.example.com:
  installer:
    username: test@vsphere.local
    password: test-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0777)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Attempt to read credentials file (should fail due to permissions)
	_, err = LoadCredentialsFile(credFilePath)
	assert.Error(t, err, "Should fail with permissions error")
	assert.Contains(t, err.Error(), "must be 0600")
}

// TestCredentialsPrecedence_InstallConfigOverFile tests install-config.yaml precedence
func TestCredentialsPrecedence_InstallConfigOverFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create credentials file with "file-" prefixed usernames
	yamlContent := `vcenter1.example.com:
  installer:
    username: file-installer@vsphere.local
    password: file-installer-password
  machine-api:
    username: file-machine-api@vsphere.local
    password: file-machine-api-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Load credentials file
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err)
	assert.NotNil(t, credsFile)

	// Create install-config ComponentCredentials with "config-" prefixed usernames
	installConfigCreds := &vsphere.ComponentCredentials{
		Installer: &vsphere.AccountCredentials{
			Username: "config-installer@vsphere.local",
			Password: "config-installer-password",
		},
		MachineAPI: &vsphere.AccountCredentials{
			Username: "config-machine-api@vsphere.local",
			Password: "config-machine-api-password",
		},
	}

	// Merge (install-config should take precedence)
	merged := MergeWithComponentCredentials(installConfigCreds, credsFile, "vcenter1.example.com")

	// Verify install-config credentials are used (NOT file credentials)
	assert.Equal(t, "config-installer@vsphere.local", merged.Installer.Username)
	assert.Equal(t, "config-installer-password", merged.Installer.Password)
	assert.Equal(t, "config-machine-api@vsphere.local", merged.MachineAPI.Username)
	assert.Equal(t, "config-machine-api-password", merged.MachineAPI.Password)
}

// TestCredentialsPrecedence_PartialInstallConfigFallbackToFile tests partial precedence
func TestCredentialsPrecedence_PartialInstallConfigFallbackToFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create credentials file with all component credentials
	yamlContent := `vcenter1.example.com:
  installer:
    username: file-installer@vsphere.local
    password: file-installer-password
  machine-api:
    username: file-machine-api@vsphere.local
    password: file-machine-api-password
  csi-driver:
    username: file-csi-driver@vsphere.local
    password: file-csi-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Load credentials file
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err)
	assert.NotNil(t, credsFile)

	// Create install-config with only installer credentials
	installConfigCreds := &vsphere.ComponentCredentials{
		Installer: &vsphere.AccountCredentials{
			Username: "config-installer@vsphere.local",
			Password: "config-installer-password",
		},
	}

	// Merge
	merged := MergeWithComponentCredentials(installConfigCreds, credsFile, "vcenter1.example.com")

	// Verify installer credentials from install-config
	assert.Equal(t, "config-installer@vsphere.local", merged.Installer.Username)
	assert.Equal(t, "config-installer-password", merged.Installer.Password)

	// Verify other component credentials from file
	assert.NotNil(t, merged.MachineAPI)
	assert.Equal(t, "file-machine-api@vsphere.local", merged.MachineAPI.Username)
	assert.Equal(t, "file-machine-api-password", merged.MachineAPI.Password)

	assert.NotNil(t, merged.CSIDriver)
	assert.Equal(t, "file-csi-driver@vsphere.local", merged.CSIDriver.Username)
	assert.Equal(t, "file-csi-password", merged.CSIDriver.Password)
}

// TestCredentialsFileFallback_MissingFile tests graceful handling when file doesn't exist
func TestCredentialsFileFallback_MissingFile(t *testing.T) {
	// Ensure credentials file does NOT exist
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials-does-not-exist")

	// Attempt to load non-existent file (should return nil, nil - graceful fallback)
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err, "Should not error when file doesn't exist")
	assert.Nil(t, credsFile, "Should return nil when file doesn't exist")

	// Provide legacy install-config credentials
	installConfigCreds := &vsphere.ComponentCredentials{
		Installer: &vsphere.AccountCredentials{
			Username: "legacy@vsphere.local",
			Password: "legacy-password",
		},
	}

	// Merge (should use install-config since file is nil)
	merged := MergeWithComponentCredentials(installConfigCreds, credsFile, "vcenter1.example.com")
	assert.Equal(t, "legacy@vsphere.local", merged.Installer.Username)
	assert.Equal(t, "legacy-password", merged.Installer.Password)
}

// TestCredentialsFileParsing_MalformedYAML tests handling of invalid YAML syntax
func TestCredentialsFileParsing_MalformedYAML(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create credentials file with malformed YAML
	malformedYAML := `vcenter1.example.com:
  installer:
    username: test@vsphere.local
      password: test-password  # incorrect indentation
  machine-api
    username: broken # missing colon
`
	err := os.WriteFile(credFilePath, []byte(malformedYAML), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Attempt to parse credentials file (should fail with YAML parsing error)
	_, err = LoadCredentialsFile(credFilePath)
	assert.Error(t, err, "Should fail with YAML parsing error")
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

// TestCredentialsFileParsing_EmptyFile tests handling of empty credentials file
func TestCredentialsFileParsing_EmptyFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create empty credentials file
	err := os.WriteFile(credFilePath, []byte(""), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Parse credentials file (should return nil - graceful fallback)
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err, "Should not error on empty file")
	assert.Nil(t, credsFile, "Should return nil for empty file")

	// Verify fallback to install-config credentials
	installConfigCreds := &vsphere.ComponentCredentials{
		Installer: &vsphere.AccountCredentials{
			Username: "config@vsphere.local",
			Password: "config-password",
		},
	}

	merged := MergeWithComponentCredentials(installConfigCreds, credsFile, "vcenter1.example.com")
	assert.Equal(t, "config@vsphere.local", merged.Installer.Username)
}

// TestCredentialsFilePartialComponents tests file with some but not all components
func TestCredentialsFilePartialComponents(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	credFilePath := filepath.Join(tmpDir, "credentials")

	// Create credentials file with only installer and machine-api
	yamlContent := `vcenter1.example.com:
  installer:
    username: file-installer@vsphere.local
    password: file-installer-password
  machine-api:
    username: file-machine-api@vsphere.local
    password: file-machine-api-password
`
	err := os.WriteFile(credFilePath, []byte(yamlContent), 0600)
	assert.NoError(t, err, "Failed to create test credentials file")

	// Load credentials file
	credsFile, err := LoadCredentialsFile(credFilePath)
	assert.NoError(t, err)
	assert.NotNil(t, credsFile)

	// Provide legacy install-config credentials (will be used for components not in file)
	installConfigCreds := &vsphere.ComponentCredentials{
		CSIDriver: &vsphere.AccountCredentials{
			Username: "legacy-csi@vsphere.local",
			Password: "legacy-csi-password",
		},
	}

	// Merge
	merged := MergeWithComponentCredentials(installConfigCreds, credsFile, "vcenter1.example.com")

	// Verify installer and machine-api from file
	assert.NotNil(t, merged.Installer)
	assert.Equal(t, "file-installer@vsphere.local", merged.Installer.Username)
	assert.Equal(t, "file-installer-password", merged.Installer.Password)

	assert.NotNil(t, merged.MachineAPI)
	assert.Equal(t, "file-machine-api@vsphere.local", merged.MachineAPI.Username)
	assert.Equal(t, "file-machine-api-password", merged.MachineAPI.Password)

	// Verify CSI from install-config (legacy fallback)
	assert.NotNil(t, merged.CSIDriver)
	assert.Equal(t, "legacy-csi@vsphere.local", merged.CSIDriver.Username)
	assert.Equal(t, "legacy-csi-password", merged.CSIDriver.Password)

	// Verify other components remain nil (will fall back to global legacy credentials at runtime)
	assert.Nil(t, merged.CloudController)
	assert.Nil(t, merged.Diagnostics)
}
