package vsphere

import (
	"context"
	"fmt"
	"sort"
	"strings"

	vim25types "github.com/vmware/govmomi/vim25/types"

	vspheretypes "github.com/openshift/installer/pkg/types/vsphere"
)

// credComponent maps a ComponentCredentials field to its display name and privilege set key.
type credComponent struct {
	name         string
	privilegeKey string
	cred         *vspheretypes.Credential
}

func componentEntries(cc *vspheretypes.ComponentCredentials) []credComponent {
	return []credComponent{
		{name: "machineAPI", privilegeKey: "machineAPI", cred: cc.MachineAPI},
		{name: "csiDriver", privilegeKey: "storage", cred: cc.CSIDriver},
		{name: "cloudController", privilegeKey: "cloudController", cred: cc.CloudController},
		{name: "diagnostics", privilegeKey: "diagnostics", cred: cc.Diagnostics},
	}
}

// PreFlightComponentCredentialValidation validates per-component vCenter credentials
// before installation begins.
//
// For each VCenter that has ComponentCredentials configured, it validates that each
// configured component's credentials have the required vSphere privileges. Components
// without credentials are skipped (they fall back to shared credentials at runtime).
//
// Returns "PerComponent" if any VCenter has ComponentCredentials configured.
// Returns "" if no VCenter has ComponentCredentials (Passthrough mode).
// Returns an error if any credential is missing required privileges, with the format:
//
//	Credential validation failed for <component> on <vcenter>: missing privileges: [<priv1> <priv2>]
func PreFlightComponentCredentialValidation(
	ctx context.Context,
	authManagerFactory func(username, password, server string) (AuthManager, error),
	platform *vspheretypes.Platform,
) (string, error) {
	hasPerComponent := false
	var errs []string

	for _, vcenter := range platform.VCenters {
		if vcenter.ComponentCredentials == nil {
			continue
		}

		hasPerComponent = true

		for _, comp := range componentEntries(vcenter.ComponentCredentials) {
			if comp.cred == nil {
				continue
			}

			mgr, err := authManagerFactory(comp.cred.User, comp.cred.Password, vcenter.Server)
			if err != nil {
				if isAuthError(err) {
					errs = append(errs, fmt.Sprintf("Authentication failed for %s on %s: %v", comp.name, vcenter.Server, err))
				} else {
					errs = append(errs, fmt.Sprintf("Failed to connect for %s on %s: %v", comp.name, vcenter.Server, err))
				}
				continue
			}

			missing, err := fetchMissingPrivileges(ctx, mgr, comp.privilegeKey, comp.cred.User)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Credential validation failed for %s on %s: %v", comp.name, vcenter.Server, err))
				continue
			}

			if len(missing) > 0 {
				errs = append(errs, fmt.Sprintf(
					"Credential validation failed for %s on %s: missing privileges: [%s]",
					comp.name, vcenter.Server, strings.Join(missing, " "),
				))
			}
		}
	}

	if len(errs) > 0 {
		return "", fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	if hasPerComponent {
		return "PerComponent", nil
	}
	return "", nil
}

// fetchMissingPrivileges queries which required privileges are absent for the given user.
func fetchMissingPrivileges(ctx context.Context, mgr AuthManager, privilegeKey, username string) ([]string, error) {
	required, ok := ComponentPrivileges[privilegeKey]
	if !ok {
		return nil, fmt.Errorf("unknown component privilege key: %s", privilegeKey)
	}

	rootRef := vim25types.ManagedObjectReference{
		Type:  "Folder",
		Value: "group-d1",
	}

	results, err := mgr.FetchUserPrivilegeOnEntities(ctx, []vim25types.ManagedObjectReference{rootRef}, username)
	if err != nil {
		return nil, err
	}

	granted := make(map[string]bool)
	for _, result := range results {
		for _, p := range result.Privileges {
			granted[p] = true
		}
	}

	var missing []string
	for _, priv := range required {
		if !granted[priv] {
			missing = append(missing, priv)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

func isAuthError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "invalidlogin") ||
		strings.Contains(lower, "unauthenticated")
}
