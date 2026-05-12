package vsphere_test

import (
	"os"
	"strings"
	"testing"
)

var guidePrivileges = []string{
	// machineAPI (19)
	"Datastore.AllocateSpace",
	"Network.Assign",
	"Resource.AssignVMToPool",
	"VirtualMachine.Config.AddExistingDisk",
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.AddRemoveDevice",
	"VirtualMachine.Config.AdvancedConfig",
	"VirtualMachine.Config.CPUCount",
	"VirtualMachine.Config.DiskExtend",
	"VirtualMachine.Config.EditDevice",
	"VirtualMachine.Config.Memory",
	"VirtualMachine.Config.RemoveDisk",
	"VirtualMachine.Config.Resource",
	"VirtualMachine.Config.Settings",
	"VirtualMachine.Interact.PowerOff",
	"VirtualMachine.Interact.PowerOn",
	"VirtualMachine.Interact.Reset",
	"VirtualMachine.Inventory.Create",
	"VirtualMachine.Inventory.Delete",
	// csiDriver (6)
	"Datastore.FileManagement",
	"StoragePod.Config",
	// cloudController (3)
	"System.Read",
	"System.View",
	// diagnostics (2)
	"Sessions.ValidateSession",
	"StorageProfile.View",
}

const guidePath = "per-component-credentials-troubleshooting.md"

func readGuide(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read %s: %v", guidePath, err)
	}
	return string(data)
}

func TestTroubleshootingGuide_CoversAllFourErrorScenarios(t *testing.T) {
	content := strings.ToLower(readGuide(t))
	scenarios := []struct {
		keyword, label string
	}{
		{"missing privilege", "missing privileges scenario"},
		{"authentication", "authentication failure scenario"},
		{"file permission", "file permissions scenario"},
		{"partial", "partial configuration scenario"},
	}
	for _, s := range scenarios {
		if !strings.Contains(content, s.keyword) {
			t.Errorf("troubleshooting guide missing %s (keyword: %q)", s.label, s.keyword)
		}
	}
}

func TestTroubleshootingGuide_HasGovcCommandOrUIPathForMissingPrivilege(t *testing.T) {
	content := readGuide(t)
	hasGovcCmd := strings.Contains(content, "govc role.update") || strings.Contains(content, "govc permissions.set")
	hasUIPath := strings.Contains(content, "Administration") && strings.Contains(content, "Access Control")
	if !hasGovcCmd && !hasUIPath {
		t.Error("troubleshooting guide lacks both govc command and vCenter UI navigation path for granting privileges")
	}
}

func TestTroubleshootingGuide_AllCanonicalPrivilegesIndexed(t *testing.T) {
	content := readGuide(t)
	for _, priv := range guidePrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("troubleshooting guide does not index privilege %q", priv)
		}
	}
}

func TestTroubleshootingGuide_AuthFailureHasCredentialCheckCommand(t *testing.T) {
	content := readGuide(t)
	if !strings.Contains(content, "govc about") && !strings.Contains(content, "GOVC_USERNAME") {
		t.Error("authentication failure section lacks govc credential verification command (e.g., govc about)")
	}
}

func TestTroubleshootingGuide_FilePermissionSectionHasChmodCommand(t *testing.T) {
	content := readGuide(t)
	if !strings.Contains(content, "chmod 0600") && !strings.Contains(content, "chmod 600") {
		t.Error("file permissions section does not provide chmod remediation command")
	}
}
