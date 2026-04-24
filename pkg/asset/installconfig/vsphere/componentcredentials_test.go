package vsphere

import (
	"testing"
)

// TestParseComponentCredentials_SingleVCenter tests credential parsing for
// a single-vCenter deployment with all component credentials provided.
func TestParseComponentCredentials_SingleVCenter(t *testing.T) {
	config := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: "pass2"},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	creds, err := ParseComponentCredentials(config)
	if err != nil {
		t.Fatalf("ParseComponentCredentials failed: %v", err)
	}

	if creds.Installer.Username != "installer@vsphere.local" {
		t.Errorf("Expected installer username 'installer@vsphere.local', got '%s'", creds.Installer.Username)
	}
	if creds.MachineAPI.Password != "pass2" {
		t.Errorf("Expected machineAPI password 'pass2', got '%s'", creds.MachineAPI.Password)
	}
}

// TestParseComponentCredentials_MultiVCenter tests credential parsing for
// a multi-vCenter deployment with per-vCenter credentials.
func TestParseComponentCredentials_MultiVCenter(t *testing.T) {
	config := &ComponentCredentials{
		Installer: &CredentialRef{
			SecretRef: &SecretReference{Name: "installer-creds", Namespace: "kube-system"},
		},
		MachineAPI: &CredentialRef{
			SecretRef: &SecretReference{Name: "machineapi-creds", Namespace: "openshift-machine-api"},
		},
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	creds, err := ParseComponentCredentials(config)
	if err != nil {
		t.Fatalf("ParseComponentCredentials failed: %v", err)
	}

	if creds.Installer.SecretRef.Name != "installer-creds" {
		t.Errorf("Expected installer secretRef name 'installer-creds', got '%s'", creds.Installer.SecretRef.Name)
	}
}

// TestParseComponentCredentials_MissingComponent tests error handling when
// a required component credential is missing.
func TestParseComponentCredentials_MissingComponent(t *testing.T) {
	config := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      nil, // Missing machine-api credentials
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	_, err := ParseComponentCredentials(config)
	if err == nil {
		t.Fatal("Expected error for missing machineAPI credentials, got nil")
	}
	if err.Error() != "machineAPI: credentials not provided" {
		t.Errorf("Expected error 'machineAPI: credentials not provided', got '%s'", err.Error())
	}
}

// TestParseComponentCredentials_MalformedCredential tests error handling for
// malformed credential entries (e.g., missing password).
func TestParseComponentCredentials_MalformedCredential(t *testing.T) {
	config := &ComponentCredentials{
		Installer:       &CredentialRef{Username: "installer@vsphere.local", Password: "pass1"},
		MachineAPI:      &CredentialRef{Username: "machineapi@vsphere.local", Password: ""}, // Missing password
		Storage:         &CredentialRef{Username: "storage@vsphere.local", Password: "pass3"},
		CloudController: &CredentialRef{Username: "cloudcontroller@vsphere.local", Password: "pass4"},
		Diagnostics:     &CredentialRef{Username: "diagnostics@vsphere.local", Password: "pass5"},
	}

	_, err := ParseComponentCredentials(config)
	if err == nil {
		t.Fatal("Expected error for missing machineAPI password, got nil")
	}
	if err.Error() != "machineAPI: password is empty and no secretRef provided" {
		t.Errorf("Expected password error, got '%s'", err.Error())
	}
}

// TestGetCredentialsForVCenter_SingleVCenter tests credential lookup for
// a single-vCenter deployment.
func TestGetCredentialsForVCenter_SingleVCenter(t *testing.T) {
	cred := &CredentialRef{
		Username: "admin@vsphere.local",
		Password: "password123",
	}

	username, password, err := GetCredentialsForVCenter("vcenter1.example.com", cred)
	if err != nil {
		t.Fatalf("GetCredentialsForVCenter failed: %v", err)
	}

	if username != "admin@vsphere.local" {
		t.Errorf("Expected username 'admin@vsphere.local', got '%s'", username)
	}
	if password != "password123" {
		t.Errorf("Expected password 'password123', got '%s'", password)
	}
}

// TestGetCredentialsForVCenter_MultiVCenter tests credential lookup for
// a multi-vCenter deployment with FQDN-based keys.
func TestGetCredentialsForVCenter_MultiVCenter(t *testing.T) {
	// Multi-vCenter credentials use secretRef
	cred := &CredentialRef{
		SecretRef: &SecretReference{
			Name:      "multi-vcenter-creds",
			Namespace: "kube-system",
		},
	}

	username, _, err := GetCredentialsForVCenter("vcenter1.example.com", cred)
	if err != nil {
		t.Fatalf("GetCredentialsForVCenter failed: %v", err)
	}

	// For secretRef, username should be FQDN-based placeholder
	expectedUsername := "vcenter1.example.com.username"
	if username != expectedUsername {
		t.Errorf("Expected username '%s', got '%s'", expectedUsername, username)
	}
}

// TestGetCredentialsForVCenter_MissingVCenter tests error handling when
// credentials are missing for a specific vCenter in multi-vCenter deployment.
func TestGetCredentialsForVCenter_MissingVCenter(t *testing.T) {
	// Empty credential ref
	cred := &CredentialRef{}

	_, _, err := GetCredentialsForVCenter("vcenter2.example.com", cred)
	if err == nil {
		t.Fatal("Expected error for missing vCenter credentials, got nil")
	}
	if err.Error() != "no credentials found for vCenter vcenter2.example.com" {
		t.Errorf("Expected 'no credentials found' error, got '%s'", err.Error())
	}
}
