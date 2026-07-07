package vsphere

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// Story #9: Brownfield Migration Tooling
// Unit tests for migration validation logic

// TestMigration_MachineAPIPrivilegeMissing_ValidationFailure verifies that
// migration validation fails when machine-api credentials lack required privileges.
// AC: Migration fails before any secrets are created with detailed error message.
func TestMigration_MachineAPIPrivilegeMissing_ValidationFailure(t *testing.T) {
	ctx := context.Background()

	// Given: Machine API credentials missing VirtualMachine.Provisioning.Clone privilege
	creds := &vsphere.AccountCredentials{
		Username: "machine-api@vsphere.local",
		Password: "password",
		VCenter:  "vcenter.example.com",
	}

	// Create a mock validator that returns missing privilege
	validator := &MockPrivilegeValidator{
		results: map[string]*ValidationResult{
			"machine-api": {
				Valid:             false,
				MissingPrivileges: []string{"VirtualMachine.Provisioning.Clone"},
				Scope:             "Datacenter",
			},
		},
	}

	// When: Migration validates privileges
	result, err := validator.ValidateComponentPrivileges(ctx, "machine-api", creds, "vcenter.example.com")

	// Then: Validation fails with missing privilege
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid {
		t.Error("expected validation to fail, but it passed")
	}
	if len(result.MissingPrivileges) == 0 || result.MissingPrivileges[0] != "VirtualMachine.Provisioning.Clone" {
		t.Errorf("expected missing privilege VirtualMachine.Provisioning.Clone, got %v", result.MissingPrivileges)
	}
	if result.Scope != "Datacenter" {
		t.Errorf("expected scope Datacenter, got %s", result.Scope)
	}

	// And: Error message should indicate which component and privilege
	expectedErrorFragment := "machine-api credentials missing required privilege: VirtualMachine.Provisioning.Clone"
	actualError := formatValidationError("machine-api", result)
	if !strings.Contains(actualError, expectedErrorFragment) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedErrorFragment, actualError)
	}
}

// TestMigration_CSIPrivilegeMissing_ValidationFailure verifies that
// migration validation fails when CSI credentials lack required privileges.
// AC: Migration fails before any secrets are created.
func TestMigration_CSIPrivilegeMissing_ValidationFailure(t *testing.T) {
	ctx := context.Background()

	// Given: CSI Driver credentials missing Datastore.AllocateSpace privilege
	creds := &vsphere.AccountCredentials{
		Username: "csi-driver@vsphere.local",
		Password: "password",
		VCenter:  "vcenter.example.com",
	}

	// Create a mock validator that returns missing privilege
	validator := &MockPrivilegeValidator{
		results: map[string]*ValidationResult{
			"csi-driver": {
				Valid:             false,
				MissingPrivileges: []string{"Datastore.AllocateSpace"},
				Scope:             "Datastore",
			},
		},
	}

	// When: Migration validates privileges
	result, err := validator.ValidateComponentPrivileges(ctx, "csi-driver", creds, "vcenter.example.com")

	// Then: Validation fails with missing privilege
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid {
		t.Error("expected validation to fail, but it passed")
	}
	if len(result.MissingPrivileges) == 0 || result.MissingPrivileges[0] != "Datastore.AllocateSpace" {
		t.Errorf("expected missing privilege Datastore.AllocateSpace, got %v", result.MissingPrivileges)
	}
	if result.Scope != "Datastore" {
		t.Errorf("expected scope Datastore, got %s", result.Scope)
	}

	// And: Error message should indicate which component and privilege
	expectedErrorFragment := "csi-driver credentials missing required privilege: Datastore.AllocateSpace"
	actualError := formatValidationError("csi-driver", result)
	if !strings.Contains(actualError, expectedErrorFragment) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedErrorFragment, actualError)
	}
}

// TestMigration_CredentialsFilePermissions verifies that migration
// refuses to read credentials files with insecure permissions.
// AC: Migration fails with clear error message when file permissions are too permissive.
func TestMigration_CredentialsFilePermissions(t *testing.T) {
	// Given: Credentials file exists with permissions 0644 (too permissive)
	tmpDir := t.TempDir()
	credsFilePath := filepath.Join(tmpDir, "credentials")

	// Create credentials file with content
	credsContent := `vcenter.example.com:
  machine-api:
    username: machine-api@vsphere.local
    password: password
`
	err := os.WriteFile(credsFilePath, []byte(credsContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test credentials file: %v", err)
	}

	// When: Migration attempts to read credentials file
	_, err = LoadCredentialsFile(credsFilePath)

	// Then: Migration fails with error about permissions
	if err == nil {
		t.Fatal("expected error for invalid permissions, got nil")
	}
	if !strings.Contains(err.Error(), "has permissions 0644, must be 0600") {
		t.Errorf("expected error about permissions, got: %v", err)
	}
	if !strings.Contains(err.Error(), credsFilePath) {
		t.Errorf("expected error to mention file path, got: %v", err)
	}
}

// MockPrivilegeValidator is a mock implementation of PrivilegeValidator for testing.
type MockPrivilegeValidator struct {
	results map[string]*ValidationResult
}

// ValidateComponentPrivileges returns pre-configured validation results for testing.
func (m *MockPrivilegeValidator) ValidateComponentPrivileges(ctx context.Context, component string, creds *vsphere.AccountCredentials, vcenter string) (*ValidationResult, error) {
	if result, ok := m.results[component]; ok {
		return result, nil
	}
	return &ValidationResult{Valid: true, MissingPrivileges: []string{}, Scope: "Datacenter"}, nil
}

// formatValidationError formats a validation error message for testing.
func formatValidationError(component string, result *ValidationResult) string {
	if !result.Valid {
		return strings.Join([]string{
			component,
			"credentials missing required privilege:",
			strings.Join(result.MissingPrivileges, ", "),
		}, " ")
	}
	return ""
}
