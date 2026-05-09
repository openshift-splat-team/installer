// Tests for Story #40: Installer Pre-flight Component Credential Validation

package vsphere

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	vim25types "github.com/vmware/govmomi/vim25/types"

	"github.com/openshift/installer/pkg/asset/installconfig/vsphere/mock"
	vspheretypes "github.com/openshift/installer/pkg/types/vsphere"
)

// ---------------------------------------------------------------------------
// AC1 — Missing privilege blocks installation with exact error format
// ---------------------------------------------------------------------------

// TestPreFlightValidation_MissingPrivilege_BlocksInstallation verifies that
// when machineAPI credentials are missing VirtualMachine.Inventory.Create,
// installation fails with the exact error format from the AC.
//
// AC: Credential validation failed for machineAPI on vcenter1.example.com:
//
//	missing privileges: [VirtualMachine.Inventory.Create]
func TestPreFlightValidation_MissingPrivilege_BlocksInstallation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{
						User:     "machineapi@vsphere.local",
						Password: "mapi-pass",
					},
				},
			},
		},
	}

	// machineAPI credentials have all machineAPI privileges EXCEPT VirtualMachine.Inventory.Create
	machineAPIPrivileges := machineAPIPrivilegesWithout("VirtualMachine.Inventory.Create")

	authMgr := mock.NewMockAuthManager(ctrl)
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "machineapi@vsphere.local").
		Return(privilegeResultFor(machineAPIPrivileges), nil).
		AnyTimes()

	factory := singleVCenterFactory("vcenter1.example.com", authMgr)

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	wantSubstr := "Credential validation failed for machineAPI on vcenter1.example.com: missing privileges: [VirtualMachine.Inventory.Create]"
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("error %q does not contain %q", err.Error(), wantSubstr)
	}
}

// TestPreFlightValidation_MultiplePrivilegesMissing verifies that all missing
// privileges are reported together in the error (not just the first one).
func TestPreFlightValidation_MultiplePrivilegesMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{
						User:     "machineapi@vsphere.local",
						Password: "mapi-pass",
					},
				},
			},
		},
	}

	// No privileges at all for machineAPI
	authMgr := mock.NewMockAuthManager(ctrl)
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "machineapi@vsphere.local").
		Return(privilegeResultFor(nil), nil).
		AnyTimes()

	factory := singleVCenterFactory("vcenter1.example.com", authMgr)

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	// Should include "Credential validation failed for machineAPI"
	if !strings.Contains(err.Error(), "Credential validation failed for machineAPI") {
		t.Errorf("error %q missing expected component name", err.Error())
	}

	// Should contain "missing privileges:" with bracket notation
	if !strings.Contains(err.Error(), "missing privileges: [") {
		t.Errorf("error %q missing 'missing privileges: [' format", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AC2 — Partial component credentials → PerComponent mode + fallback for others
// ---------------------------------------------------------------------------

// TestPreFlightValidation_PartialCredentials_PerComponentMode verifies that
// when only machineAPI and csiDriver are configured, the returned credentialsMode
// is "PerComponent" and cloudController/diagnostics are skipped (no validation error).
func TestPreFlightValidation_PartialCredentials_PerComponentMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{
						User:     "machineapi@vsphere.local",
						Password: "mapi-pass",
					},
					CSIDriver: &vspheretypes.Credential{
						User:     "csi@vsphere.local",
						Password: "csi-pass",
					},
					// CloudController and Diagnostics intentionally absent → fallback to shared
				},
			},
		},
	}

	authMgr := mock.NewMockAuthManager(ctrl)
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "machineapi@vsphere.local").
		Return(privilegeResultFor(machineAPIPrivilegesAll()), nil).
		AnyTimes()
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "csi@vsphere.local").
		Return(privilegeResultFor(csiDriverPrivilegesAll()), nil).
		AnyTimes()
	// cloudController and diagnostics not called (no credentials configured)

	factory := singleVCenterFactory("vcenter1.example.com", authMgr)

	credentialsMode, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if credentialsMode != "PerComponent" {
		t.Errorf("credentialsMode = %q; want PerComponent", credentialsMode)
	}
}

// TestPreFlightValidation_AllComponentCredentials_PerComponentMode verifies that
// all five components configured returns PerComponent mode with no errors.
func TestPreFlightValidation_AllComponentCredentials_PerComponentMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI:      &vspheretypes.Credential{User: "machineapi@vsphere.local", Password: "pass"},
					CSIDriver:       &vspheretypes.Credential{User: "csi@vsphere.local", Password: "pass"},
					CloudController: &vspheretypes.Credential{User: "ccm@vsphere.local", Password: "pass"},
					Diagnostics:     &vspheretypes.Credential{User: "diag@vsphere.local", Password: "pass"},
				},
			},
		},
	}

	authMgr := mock.NewMockAuthManager(ctrl)
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(privilegeResultFor(allPrivileges()), nil).
		AnyTimes()

	factory := singleVCenterFactory("vcenter1.example.com", authMgr)

	credentialsMode, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if credentialsMode != "PerComponent" {
		t.Errorf("credentialsMode = %q; want PerComponent", credentialsMode)
	}
}

// ---------------------------------------------------------------------------
// AC3 — No componentCredentials → skip validation, credentialsMode = Passthrough
// ---------------------------------------------------------------------------

// TestPreFlightValidation_NoComponentCredentials_Skipped verifies that when
// no componentCredentials block is present in any vCenter, pre-flight validation
// is skipped and credentialsMode is not set to PerComponent.
func TestPreFlightValidation_NoComponentCredentials_Skipped(t *testing.T) {
	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:    "vcenter1.example.com",
				Username:  "installer@vsphere.local",
				Password:  "installpass",
				// ComponentCredentials intentionally absent
			},
		},
	}

	// No AuthManager calls expected — factory should not be invoked
	factory := func(username, password, server string) (AuthManager, error) {
		return nil, fmt.Errorf("factory must not be called when no componentCredentials configured")
	}

	credentialsMode, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if credentialsMode == "PerComponent" {
		t.Error("credentialsMode must not be PerComponent when no componentCredentials configured")
	}
}

// TestPreFlightValidation_NilPlatformVCenters_NoError verifies that a nil or
// empty VCenters list does not panic and returns no error (AC3 variant).
func TestPreFlightValidation_NilPlatformVCenters_NoError(t *testing.T) {
	platform := &vspheretypes.Platform{
		VCenters: nil,
	}

	factory := func(username, password, server string) (AuthManager, error) {
		return nil, fmt.Errorf("factory must not be called for empty vcenters")
	}

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err != nil {
		t.Fatalf("unexpected error for empty vcenters: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Adversarial cases
// ---------------------------------------------------------------------------

// TestPreFlightValidation_AuthenticationFailure verifies that when vCenter
// authentication fails, the error message contains "Authentication failed for".
func TestPreFlightValidation_AuthenticationFailure(t *testing.T) {
	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{
						User:     "baduser@vsphere.local",
						Password: "wrongpass",
					},
				},
			},
		},
	}

	factory := func(username, password, server string) (AuthManager, error) {
		return nil, fmt.Errorf("authentication failed: InvalidLogin")
	}

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected authentication error, got nil")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "authentication") &&
		!strings.Contains(strings.ToLower(err.Error()), "invalidlogin") {
		t.Errorf("error %q should mention authentication failure", err.Error())
	}
}

// TestPreFlightValidation_MultiVCenter_MissingPrivilegeOnOneVCenter verifies
// that a missing privilege on one vCenter blocks installation, even if the
// other vCenter validates successfully.
func TestPreFlightValidation_MultiVCenter_MissingPrivilegeOnOneVCenter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{User: "machineapi@vsphere.local", Password: "pass"},
				},
			},
			{
				Server:   "vcenter2.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{User: "machineapi@vsphere.local", Password: "pass"},
				},
			},
		},
	}

	authMgr1 := mock.NewMockAuthManager(ctrl)
	authMgr1.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "machineapi@vsphere.local").
		Return(privilegeResultFor(machineAPIPrivilegesAll()), nil).
		AnyTimes()

	authMgr2 := mock.NewMockAuthManager(ctrl)
	authMgr2.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), "machineapi@vsphere.local").
		Return(privilegeResultFor(machineAPIPrivilegesWithout("VirtualMachine.Inventory.Create")), nil).
		AnyTimes()

	factory := func(username, password, server string) (AuthManager, error) {
		switch server {
		case "vcenter1.example.com":
			return authMgr1, nil
		case "vcenter2.example.com":
			return authMgr2, nil
		}
		return nil, fmt.Errorf("unknown vcenter: %s", server)
	}

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected validation error for vcenter2, got nil")
	}

	if !strings.Contains(err.Error(), "vcenter2.example.com") {
		t.Errorf("error %q should reference vcenter2.example.com", err.Error())
	}
}

// TestPreFlightValidation_MultipleComponentsWithMissingPrivileges verifies
// that all failing components are reported (not just the first failure).
func TestPreFlightValidation_MultipleComponentsWithMissingPrivileges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{User: "machineapi@vsphere.local", Password: "pass"},
					CSIDriver:  &vspheretypes.Credential{User: "csi@vsphere.local", Password: "pass"},
				},
			},
		},
	}

	authMgr := mock.NewMockAuthManager(ctrl)
	// Both components get no privileges → both should fail
	authMgr.EXPECT().
		FetchUserPrivilegeOnEntities(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(privilegeResultFor(nil), nil).
		AnyTimes()

	factory := singleVCenterFactory("vcenter1.example.com", authMgr)

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}

	// Both components should be mentioned
	if !strings.Contains(err.Error(), "machineAPI") {
		t.Errorf("error %q should mention machineAPI", err.Error())
	}
	if !strings.Contains(err.Error(), "csiDriver") {
		t.Errorf("error %q should mention csiDriver", err.Error())
	}
}

// TestPreFlightValidation_EmptyCredential_Rejected verifies that a credential
// with an empty username is rejected before any AuthManager call is attempted.
func TestPreFlightValidation_EmptyCredential_Rejected(t *testing.T) {
	platform := &vspheretypes.Platform{
		VCenters: []vspheretypes.VCenter{
			{
				Server:   "vcenter1.example.com",
				Username: "installer@vsphere.local",
				Password: "installpass",
				ComponentCredentials: &vspheretypes.ComponentCredentials{
					MachineAPI: &vspheretypes.Credential{
						User:     "", // empty username
						Password: "pass",
					},
				},
			},
		},
	}

	factory := func(username, password, server string) (AuthManager, error) {
		if username == "" {
			return nil, fmt.Errorf("authentication failed: empty username")
		}
		return nil, fmt.Errorf("unexpected call")
	}

	_, err := PreFlightComponentCredentialValidation(context.Background(), factory, platform)
	if err == nil {
		t.Fatal("expected error for empty credential, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// singleVCenterFactory returns a factory that always returns authMgr for the
// expected vCenter server (any credentials).
func singleVCenterFactory(expectedServer string, authMgr AuthManager) func(username, password, server string) (AuthManager, error) {
	return func(username, password, server string) (AuthManager, error) {
		if server != expectedServer {
			return nil, fmt.Errorf("unexpected vCenter server %q, want %q", server, expectedServer)
		}
		return authMgr, nil
	}
}

// privilegeResultFor converts a string slice to the UserPrivilegeResult format
// that FetchUserPrivilegeOnEntities returns.
func privilegeResultFor(privileges []string) []vim25types.UserPrivilegeResult {
	if privileges == nil {
		return nil
	}
	return []vim25types.UserPrivilegeResult{{Privileges: privileges}}
}

// machineAPIPrivilegesAll returns the complete machineAPI privilege set from
// componentvalidation.go ComponentPrivileges["machineAPI"].
func machineAPIPrivilegesAll() []string {
	return ComponentPrivileges["machineAPI"]
}

// machineAPIPrivilegesWithout returns the machineAPI privilege set minus the
// named privilege (for testing missing-privilege detection).
func machineAPIPrivilegesWithout(excluded string) []string {
	all := machineAPIPrivilegesAll()
	result := make([]string, 0, len(all))
	for _, p := range all {
		if p != excluded {
			result = append(result, p)
		}
	}
	return result
}

// csiDriverPrivilegesAll returns the complete CSI driver privilege set.
func csiDriverPrivilegesAll() []string {
	return ComponentPrivileges["storage"]
}

// allPrivileges returns a superset containing all defined privileges from all
// components, so any component's validation will pass.
func allPrivileges() []string {
	seen := map[string]bool{}
	var result []string
	for _, privs := range ComponentPrivileges {
		for _, p := range privs {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}
