package percomponentcredentials_test

import (
	"os"
	"strings"
	"testing"
)

var machineAPIPrivileges = []string{
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
}

var csiDriverPrivileges = []string{
	"Datastore.AllocateSpace",
	"Datastore.FileManagement",
	"StoragePod.Config",
	"VirtualMachine.Config.AddExistingDisk",
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.RemoveDisk",
}

var cloudControllerPrivileges = []string{
	"System.Read",
	"System.View",
	"VirtualMachine.Inventory.Create",
}

var diagnosticsPrivileges = []string{
	"Sessions.ValidateSession",
	"StorageProfile.View",
}

func readScript(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

const govcScript = "create-roles.sh"

func TestGovcScript_MachineAPI_RoleExists(t *testing.T) {
	if !strings.Contains(readScript(t, govcScript), "openshift-vsphere-machineapi") {
		t.Error("govc script does not define role openshift-vsphere-machineapi")
	}
}

func TestGovcScript_CSIDriver_RoleExists(t *testing.T) {
	if !strings.Contains(readScript(t, govcScript), "openshift-vsphere-csidriver") {
		t.Error("govc script does not define role openshift-vsphere-csidriver")
	}
}

func TestGovcScript_CloudController_RoleExists(t *testing.T) {
	if !strings.Contains(readScript(t, govcScript), "openshift-vsphere-cloudcontroller") {
		t.Error("govc script does not define role openshift-vsphere-cloudcontroller")
	}
}

func TestGovcScript_Diagnostics_RoleExists(t *testing.T) {
	if !strings.Contains(readScript(t, govcScript), "openshift-vsphere-diagnostics") {
		t.Error("govc script does not define role openshift-vsphere-diagnostics")
	}
}

func TestGovcScript_MachineAPI_AllPrivilegesPresent(t *testing.T) {
	content := readScript(t, govcScript)
	for _, priv := range machineAPIPrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("govc script missing machineAPI privilege %q", priv)
		}
	}
}

func TestGovcScript_CSIDriver_AllPrivilegesPresent(t *testing.T) {
	content := readScript(t, govcScript)
	for _, priv := range csiDriverPrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("govc script missing csiDriver privilege %q", priv)
		}
	}
}

func TestGovcScript_CloudController_AllPrivilegesPresent(t *testing.T) {
	content := readScript(t, govcScript)
	for _, priv := range cloudControllerPrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("govc script missing cloudController privilege %q", priv)
		}
	}
}

func TestGovcScript_Diagnostics_AllPrivilegesPresent(t *testing.T) {
	content := readScript(t, govcScript)
	for _, priv := range diagnosticsPrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("govc script missing diagnostics privilege %q", priv)
		}
	}
}

const powercliScript = "create-roles.ps1"

func TestPowerCLIScript_AllFourRolesPresent(t *testing.T) {
	content := readScript(t, powercliScript)
	for _, name := range []string{
		"openshift-vsphere-machineapi",
		"openshift-vsphere-csidriver",
		"openshift-vsphere-cloudcontroller",
		"openshift-vsphere-diagnostics",
	} {
		if !strings.Contains(content, name) {
			t.Errorf("PowerCLI script missing role name %q", name)
		}
	}
}

func TestPowerCLIScript_MachineAPI_AllPrivilegesPresent(t *testing.T) {
	content := readScript(t, powercliScript)
	for _, priv := range machineAPIPrivileges {
		if !strings.Contains(content, priv) {
			t.Errorf("PowerCLI script missing machineAPI privilege %q", priv)
		}
	}
}
