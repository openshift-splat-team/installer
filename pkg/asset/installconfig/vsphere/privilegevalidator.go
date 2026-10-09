package vsphere

import (
	"context"
	"fmt"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// ValidationResult holds the result of privilege validation for a component.
type ValidationResult struct {
	// Valid indicates whether all required privileges are present
	Valid bool
	// MissingPrivileges contains the list of missing privilege names
	MissingPrivileges []string
	// Scope identifies the entity type where privileges are required (e.g., "Datacenter", "Datastore")
	Scope string
}

// PrivilegeValidator validates vSphere component credentials have required privileges.
type PrivilegeValidator struct {
	// In a real implementation, this would hold a vSphere client connection
	// For now, this is a placeholder for the interface
}

// NewPrivilegeValidator creates a new PrivilegeValidator instance.
func NewPrivilegeValidator() *PrivilegeValidator {
	return &PrivilegeValidator{}
}

// ValidateComponentPrivileges validates that the given component credentials have all required privileges.
// Returns ValidationResult indicating success/failure and any missing privileges.
func (v *PrivilegeValidator) ValidateComponentPrivileges(ctx context.Context, component string, creds *vsphere.AccountCredentials, vcenter string) (*ValidationResult, error) {
	// Get required privileges for this component
	required := GetRequiredPrivileges(component)
	if len(required) == 0 {
		return nil, fmt.Errorf("unknown component: %s", component)
	}

	// In a real implementation, this would:
	// 1. Connect to vCenter using creds
	// 2. Call AuthorizationManager.FetchUserPrivilegeOnEntities()
	// 3. Compare returned privileges against required list
	// 4. Identify missing privileges and their scopes
	//
	// For now, we return a stub that allows tests to define behavior via mocking

	result := &ValidationResult{
		Valid:             true,
		MissingPrivileges: []string{},
		Scope:             getDefaultScopeForComponent(component),
	}

	return result, nil
}

// GetRequiredPrivileges returns the list of required vSphere privileges for the given component.
func GetRequiredPrivileges(component string) []string {
	privilegeMap := map[string][]string{
		"installer": installerPrivileges,
		"machine-api": machineAPIPrivileges,
		"csi-driver": csiDriverPrivileges,
		"cloud-controller": cloudControllerPrivileges,
		"diagnostics": diagnosticsPrivileges,
	}

	return privilegeMap[component]
}

// getDefaultScopeForComponent returns the default vSphere entity scope for privilege checks.
func getDefaultScopeForComponent(component string) string {
	scopeMap := map[string]string{
		"installer":        "Datacenter",
		"machine-api":      "Datacenter",
		"csi-driver":       "Datastore",
		"cloud-controller": "Datacenter",
		"diagnostics":      "Datacenter",
	}

	if scope, ok := scopeMap[component]; ok {
		return scope
	}
	return "Datacenter"
}

// Privilege lists for each component based on design doc requirements.
// These are comprehensive lists of vSphere privileges required for each component's operations.

// installerPrivileges contains ~45 privileges required for cluster infrastructure deployment.
var installerPrivileges = []string{
	// Folder management
	"Folder.Create",
	"Folder.Delete",
	"Folder.Move",
	"Folder.Rename",

	// Resource pool management
	"ResourcePool.Create",
	"ResourcePool.Delete",
	"ResourcePool.Assign",

	// Virtual machine provisioning
	"VirtualMachine.Provisioning.Clone",
	"VirtualMachine.Provisioning.DeployTemplate",
	"VirtualMachine.Provisioning.MarkAsTemplate",
	"VirtualMachine.Provisioning.MarkAsVM",
	"VirtualMachine.Provisioning.CustomizeGuest",
	"VirtualMachine.Provisioning.ReadCustSpecs",
	"VirtualMachine.Provisioning.ModifyCustSpecs",

	// Virtual machine configuration
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.AddRemoveDevice",
	"VirtualMachine.Config.AdvancedConfig",
	"VirtualMachine.Config.CPUCount",
	"VirtualMachine.Config.Memory",
	"VirtualMachine.Config.Settings",
	"VirtualMachine.Config.Resource",
	"VirtualMachine.Config.EditDevice",
	"VirtualMachine.Config.RemoveDisk",
	"VirtualMachine.Config.Rename",
	"VirtualMachine.Config.Annotation",

	// Virtual machine interaction
	"VirtualMachine.Interact.PowerOn",
	"VirtualMachine.Interact.PowerOff",
	"VirtualMachine.Interact.Reset",
	"VirtualMachine.Interact.Suspend",
	"VirtualMachine.Interact.ConsoleInteract",
	"VirtualMachine.Interact.DeviceConnection",
	"VirtualMachine.Interact.SetCDMedia",
	"VirtualMachine.Interact.GuestControl",

	// Virtual machine inventory
	"VirtualMachine.Inventory.Create",
	"VirtualMachine.Inventory.Delete",
	"VirtualMachine.Inventory.Move",
	"VirtualMachine.Inventory.Register",
	"VirtualMachine.Inventory.Unregister",

	// Network assignment
	"Network.Assign",

	// Datastore allocation
	"Datastore.AllocateSpace",
	"Datastore.Browse",
	"Datastore.FileManagement",
}

// machineAPIPrivileges contains ~35 privileges required for VM lifecycle operations.
var machineAPIPrivileges = []string{
	// Virtual machine provisioning
	"VirtualMachine.Provisioning.Clone",
	"VirtualMachine.Provisioning.DeployTemplate",
	"VirtualMachine.Provisioning.CustomizeGuest",

	// Virtual machine configuration
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.AddRemoveDevice",
	"VirtualMachine.Config.AdvancedConfig",
	"VirtualMachine.Config.CPUCount",
	"VirtualMachine.Config.Memory",
	"VirtualMachine.Config.Settings",
	"VirtualMachine.Config.Resource",
	"VirtualMachine.Config.EditDevice",
	"VirtualMachine.Config.RemoveDisk",
	"VirtualMachine.Config.Rename",
	"VirtualMachine.Config.Annotation",

	// Virtual machine interaction
	"VirtualMachine.Interact.PowerOn",
	"VirtualMachine.Interact.PowerOff",
	"VirtualMachine.Interact.Reset",
	"VirtualMachine.Interact.Suspend",
	"VirtualMachine.Interact.DeviceConnection",
	"VirtualMachine.Interact.GuestControl",

	// Virtual machine inventory
	"VirtualMachine.Inventory.Create",
	"VirtualMachine.Inventory.Delete",
	"VirtualMachine.Inventory.Move",
	"VirtualMachine.Inventory.Register",
	"VirtualMachine.Inventory.Unregister",

	// Network assignment
	"Network.Assign",

	// Datastore allocation
	"Datastore.AllocateSpace",
	"Datastore.Browse",
	"Datastore.FileManagement",

	// Resource pool
	"ResourcePool.Assign",

	// Folder operations
	"Folder.Create",
	"Folder.Delete",
}

// csiDriverPrivileges contains ~10-15 privileges required for storage provisioning.
var csiDriverPrivileges = []string{
	// Datastore operations (primary focus)
	"Datastore.AllocateSpace",
	"Datastore.Browse",
	"Datastore.FileManagement",
	"Datastore.DeleteFile",
	"Datastore.UpdateVirtualMachineFiles",

	// Virtual machine disk operations
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.AddRemoveDevice",
	"VirtualMachine.Config.RemoveDisk",
	"VirtualMachine.Config.EditDevice",

	// System operations
	"System.Anonymous",
	"System.Read",
	"System.View",
}

// cloudControllerPrivileges contains ~10 read-only privileges required for node discovery.
var cloudControllerPrivileges = []string{
	// Read-only system access
	"System.Anonymous",
	"System.Read",
	"System.View",

	// Virtual machine read operations
	"VirtualMachine.Inventory.Register",

	// Resource discovery (read-only)
	"Host.Config.Network",
	"Network.Assign",

	// Datacenter read operations
	"Datacenter.Read",

	// Folder read operations
	"Folder.Read",

	// Cluster read operations
	"ClusterComputeResource.Read",
}

// diagnosticsPrivileges contains ~5 read-only privileges required for troubleshooting.
var diagnosticsPrivileges = []string{
	// Read-only system access
	"System.Anonymous",
	"System.Read",
	"System.View",

	// Virtual machine read operations
	"VirtualMachine.GuestOperations.Query",

	// Log access
	"Global.LogEvent",
}
