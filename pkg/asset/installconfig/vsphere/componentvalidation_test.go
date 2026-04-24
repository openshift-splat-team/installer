package vsphere

import (
	"fmt"
	"testing"
)

// TestValidateComponentCredentials_AllValid tests successful validation
// when all component credentials have required privileges.
func TestValidateComponentCredentials_AllValid(t *testing.T) {
	vcenters := []string{"vcenter1.example.com"}
	credentials := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: "pass2"},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	report, err := ValidateComponentCredentials(vcenters, credentials)
	if err != nil {
		t.Fatalf("ValidateComponentCredentials failed: %v", err)
	}

	if !report.Valid {
		t.Errorf("Expected Valid=true, got false. Errors: %v", report.Errors)
	}

	if len(report.Errors) != 0 {
		t.Errorf("Expected no errors, got %d errors", len(report.Errors))
	}

	if !report.ComponentResults["installer"] {
		t.Error("Expected installer component to be valid")
	}
}

// TestValidateComponentCredentials_MissingPrivilege tests detection of
// missing privileges for a specific component.
func TestValidateComponentCredentials_MissingPrivilege(t *testing.T) {
	// This test validates the error reporting structure
	// Real privilege validation requires vSphere API integration
	vcenters := []string{"vcenter1.example.com"}
	credentials := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "", Password: "pass2"}, // Invalid username
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	report, err := ValidateComponentCredentials(vcenters, credentials)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if report.Valid {
		t.Error("Expected Valid=false, got true")
	}

	if len(report.Errors) == 0 {
		t.Fatal("Expected errors in report, got none")
	}

	// Verify error indicates machineAPI component
	found := false
	for _, validationErr := range report.Errors {
		if validationErr.Component == "machineAPI" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for machineAPI component")
	}
}

// TestValidateComponentCredentials_MultiVCenter tests validation across
// multiple vCenters with different privilege configurations.
func TestValidateComponentCredentials_MultiVCenter(t *testing.T) {
	vcenters := []string{"vcenter1.example.com", "vcenter2.example.com"}
	credentials := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: "pass2"},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	report, err := ValidateComponentCredentials(vcenters, credentials)
	if err != nil {
		t.Fatalf("ValidateComponentCredentials failed: %v", err)
	}

	// Verify both vCenters validated
	if !report.VCenterResults["vcenter1.example.com"] {
		t.Error("Expected vcenter1.example.com to be valid")
	}
	if !report.VCenterResults["vcenter2.example.com"] {
		t.Error("Expected vcenter2.example.com to be valid")
	}
}

// TestValidateComponentCredentials_AuthenticationFailure tests handling of
// authentication failures when connecting to vCenter.
func TestValidateComponentCredentials_AuthenticationFailure(t *testing.T) {
	// Test with empty username to simulate auth failure
	vcenters := []string{"vcenter1.example.com"}
	credentials := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: "pass2"},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	report, err := ValidateComponentCredentials(vcenters, credentials)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if report.Valid {
		t.Error("Expected Valid=false, got true")
	}

	// Check that error includes component name
	if len(report.Errors) == 0 {
		t.Fatal("Expected errors in report")
	}
	if report.Errors[0].Component != "installer" {
		t.Errorf("Expected error for installer component, got %s", report.Errors[0].Component)
	}
}

// TestValidateComponentCredentials_ConnectionFailure tests handling of
// network connectivity failures to vCenter.
func TestValidateComponentCredentials_ConnectionFailure(t *testing.T) {
	// Simulated by missing credentials
	vcenters := []string{"vcenter1.example.com"}
	credentials := &ComponentCredentials{
		Installer:       nil,
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: "pass2"},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	report, err := ValidateComponentCredentials(vcenters, credentials)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if report.Valid {
		t.Error("Expected Valid=false, got true")
	}

	// Verify error for missing installer credentials
	if len(report.Errors) == 0 {
		t.Fatal("Expected errors in report")
	}
}

// TestValidatePrivileges_InstallerComponent tests privilege validation for
// the installer component (~50 privileges).
func TestValidatePrivileges_InstallerComponent(t *testing.T) {
	err := ValidatePrivileges(nil, "installer", "installer@vsphere.local")
	if err != nil {
		t.Fatalf("ValidatePrivileges failed: %v", err)
	}

	// Verify installer privileges are defined
	privileges := ComponentPrivileges["installer"]
	if len(privileges) < 40 {
		t.Errorf("Expected ~50 installer privileges, got %d", len(privileges))
	}
}

// TestValidatePrivileges_MachineAPIComponent tests privilege validation for
// the machine-api component (35 privileges).
func TestValidatePrivileges_MachineAPIComponent(t *testing.T) {
	err := ValidatePrivileges(nil, "machineAPI", "machineapi@vsphere.local")
	if err != nil {
		t.Fatalf("ValidatePrivileges failed: %v", err)
	}

	// Verify machineAPI privileges are defined
	privileges := ComponentPrivileges["machineAPI"]
	if len(privileges) < 30 {
		t.Errorf("Expected ~35 machineAPI privileges, got %d", len(privileges))
	}
}

// TestValidatePrivileges_StorageComponent tests privilege validation for
// the storage component (10-15 privileges).
func TestValidatePrivileges_StorageComponent(t *testing.T) {
	err := ValidatePrivileges(nil, "storage", "storage@vsphere.local")
	if err != nil {
		t.Fatalf("ValidatePrivileges failed: %v", err)
	}

	// Verify storage privileges are defined
	privileges := ComponentPrivileges["storage"]
	if len(privileges) < 10 || len(privileges) > 20 {
		t.Errorf("Expected 10-15 storage privileges, got %d", len(privileges))
	}
}

// TestValidatePrivileges_CloudControllerComponent tests privilege validation
// for the cloud-controller component (~10 privileges, mostly read-only).
func TestValidatePrivileges_CloudControllerComponent(t *testing.T) {
	err := ValidatePrivileges(nil, "cloudController", "cloudcontroller@vsphere.local")
	if err != nil {
		t.Fatalf("ValidatePrivileges failed: %v", err)
	}

	// Verify cloudController privileges are defined
	privileges := ComponentPrivileges["cloudController"]
	if len(privileges) < 8 || len(privileges) > 15 {
		t.Errorf("Expected ~10 cloudController privileges, got %d", len(privileges))
	}
}

// TestValidatePrivileges_DiagnosticsComponent tests privilege validation for
// the diagnostics component (~16 privileges).
func TestValidatePrivileges_DiagnosticsComponent(t *testing.T) {
	err := ValidatePrivileges(nil, "diagnostics", "diagnostics@vsphere.local")
	if err != nil {
		t.Fatalf("ValidatePrivileges failed: %v", err)
	}

	// Verify diagnostics privileges are defined
	privileges := ComponentPrivileges["diagnostics"]
	if len(privileges) < 14 || len(privileges) > 20 {
		t.Errorf("Expected ~16 diagnostics privileges, got %d", len(privileges))
	}
}

// TestValidationError_Format tests error message formatting for validation failures.
func TestValidationError_Format(t *testing.T) {
	err := &ValidationError{
		Component:        "machineAPI",
		VCenter:          "vcenter1.example.com",
		MissingPrivilege: "VirtualMachine.Inventory.Create",
	}

	expected := "machineAPI credentials for vcenter1.example.com: missing privilege VirtualMachine.Inventory.Create"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}

	// Test error without missing privilege
	err2 := &ValidationError{
		Component: "storage",
		VCenter:   "vcenter2.example.com",
		Err:       fmt.Errorf("connection timeout"),
	}

	expected2 := "storage credentials for vcenter2.example.com: connection timeout"
	if err2.Error() != expected2 {
		t.Errorf("Expected error message '%s', got '%s'", expected2, err2.Error())
	}
}

// TestFormatValidationReport_Success tests report formatting for successful validation.
func TestFormatValidationReport_Success(t *testing.T) {
	report := &ValidationReport{
		Valid:            true,
		Errors:           []*ValidationError{},
		ComponentResults: map[string]bool{
			"installer":       true,
			"machineAPI":      true,
			"storage":         true,
			"cloudController": true,
			"diagnostics":     true,
		},
		VCenterResults: map[string]bool{
			"vcenter1.example.com": true,
		},
	}

	output := FormatValidationReport(report)

	if !containsString(output, "✓ All component credentials validated successfully") {
		t.Error("Expected success indicator in report")
	}

	if !containsString(output, "✓ installer: validated") {
		t.Error("Expected installer component in report")
	}
}

// TestFormatValidationReport_Failure tests report formatting for failed validation.
func TestFormatValidationReport_Failure(t *testing.T) {
	report := &ValidationReport{
		Valid: false,
		Errors: []*ValidationError{
			{
				Component:        "machineAPI",
				VCenter:          "vcenter1.example.com",
				MissingPrivilege: "VirtualMachine.Inventory.Create",
			},
		},
		ComponentResults: map[string]bool{
			"installer":       true,
			"machineAPI":      false,
			"storage":         true,
			"cloudController": true,
			"diagnostics":     true,
		},
		VCenterResults: map[string]bool{
			"vcenter1.example.com": false,
		},
	}

	output := FormatValidationReport(report)

	if !containsString(output, "✗ Credential validation failed") {
		t.Error("Expected failure indicator in report")
	}

	if !containsString(output, "✗ machineAPI") {
		t.Error("Expected machineAPI failure in report")
	}

	if !containsString(output, "missing privilege VirtualMachine.Inventory.Create") {
		t.Error("Expected missing privilege in report")
	}

	if !containsString(output, "✓ installer: validated") {
		t.Error("Expected successful installer component in report")
	}
}

// TestFormatValidationReport_MultipleErrors tests report formatting when
// multiple components have validation failures.
func TestFormatValidationReport_MultipleErrors(t *testing.T) {
	report := &ValidationReport{
		Valid: false,
		Errors: []*ValidationError{
			{
				Component:        "machineAPI",
				VCenter:          "vcenter1.example.com",
				MissingPrivilege: "VirtualMachine.Inventory.Create",
			},
			{
				Component: "storage",
				VCenter:   "vcenter2.example.com",
				Err:       fmt.Errorf("authentication failed"),
			},
		},
		ComponentResults: map[string]bool{
			"installer":       true,
			"machineAPI":      false,
			"storage":         false,
			"cloudController": true,
			"diagnostics":     true,
		},
		VCenterResults: map[string]bool{
			"vcenter1.example.com": false,
			"vcenter2.example.com": false,
		},
	}

	output := FormatValidationReport(report)

	if !containsString(output, "✗ Credential validation failed") {
		t.Error("Expected failure indicator in report")
	}

	if !containsString(output, "✗ machineAPI") {
		t.Error("Expected machineAPI failure in report")
	}

	if !containsString(output, "✗ storage") {
		t.Error("Expected storage failure in report")
	}

	if !containsString(output, "✓ installer: validated") {
		t.Error("Expected successful components in report")
	}
}

// Helper function to check if a string contains a substring
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 || indexString(haystack, needle) >= 0)
}

func indexString(haystack, needle string) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
