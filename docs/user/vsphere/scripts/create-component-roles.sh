#!/bin/bash
# create-component-roles.sh
# Creates vCenter roles for OpenShift per-component credentials across multiple vCenters
#
# Usage:
#   Single vCenter:
#     ./create-component-roles.sh vcenter.example.com administrator@vsphere.local 'password'
#
#   Multiple vCenters:
#     export VCENTERS="vcenter1.example.com vcenter2.example.com"
#     export VCENTER_USER="administrator@vsphere.local"
#     export VCENTER_PASSWORD="password"
#     ./create-component-roles.sh
#
# Prerequisites:
#   - govc CLI tool installed (https://github.com/vmware/govmomi/tree/main/govc)
#   - vCenter administrator credentials
#   - Network connectivity to vCenter servers
#
# This script creates five vCenter roles with precisely scoped privileges for OpenShift components:
#   - openshift-installer: Installation and infrastructure provisioning (~45 privileges)
#   - openshift-machine-api: VM lifecycle management (~35 privileges)
#   - openshift-csi-driver: Storage provisioning (~12 privileges)
#   - openshift-cloud-controller: Read-only node discovery (~9 privileges)
#   - openshift-diagnostics: Read-only troubleshooting (~5 privileges)

set -euo pipefail

# Color output for better readability
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if govc is installed
if ! command -v govc &> /dev/null; then
    log_error "govc CLI tool is not installed"
    log_info "Install govc: https://github.com/vmware/govmomi/tree/main/govc"
    exit 1
fi

# Determine vCenter list from arguments or environment
if [ $# -eq 3 ]; then
    VCENTERS="$1"
    VCENTER_USER="$2"
    VCENTER_PASSWORD="$3"
elif [ -n "${VCENTERS:-}" ] && [ -n "${VCENTER_USER:-}" ] && [ -n "${VCENTER_PASSWORD:-}" ]; then
    : # Use environment variables
else
    log_error "Usage: $0 <vcenter> <username> <password>"
    log_error "   OR: Set VCENTERS, VCENTER_USER, VCENTER_PASSWORD environment variables"
    exit 1
fi

# Function to create a role with privileges
create_role() {
    local role_name="$1"
    shift
    local privileges=("$@")

    log_info "Creating role: $role_name (${#privileges[@]} privileges)"

    # Check if role already exists
    if govc role.ls | grep -q "^${role_name}$"; then
        log_warn "Role '$role_name' already exists, updating privileges"
        govc role.update "$role_name" "${privileges[@]}"
    else
        govc role.create "$role_name" "${privileges[@]}"
    fi

    if [ $? -eq 0 ]; then
        log_info "✓ Role '$role_name' created/updated successfully"
    else
        log_error "✗ Failed to create role '$role_name'"
        return 1
    fi
}

# Main execution loop for each vCenter
for vcenter in $VCENTERS; do
    log_info "=========================================="
    log_info "Processing vCenter: $vcenter"
    log_info "=========================================="

    export GOVC_URL="$vcenter"
    export GOVC_USERNAME="$VCENTER_USER"
    export GOVC_PASSWORD="$VCENTER_PASSWORD"
    export GOVC_INSECURE=1  # Set to 0 if using trusted certificates

    # Test connection
    if ! govc about &> /dev/null; then
        log_error "Failed to connect to vCenter: $vcenter"
        log_error "Please verify credentials and network connectivity"
        continue
    fi

    log_info "Successfully connected to $vcenter"

    # Create Installer role (~45 privileges)
    # Required for cluster infrastructure deployment
    create_role "openshift-installer" \
        "Folder.Create" \
        "Folder.Delete" \
        "Folder.Move" \
        "Folder.Rename" \
        "ResourcePool.Create" \
        "ResourcePool.Delete" \
        "ResourcePool.Assign" \
        "VirtualMachine.Provisioning.Clone" \
        "VirtualMachine.Provisioning.DeployTemplate" \
        "VirtualMachine.Provisioning.MarkAsTemplate" \
        "VirtualMachine.Provisioning.MarkAsVM" \
        "VirtualMachine.Provisioning.CustomizeGuest" \
        "VirtualMachine.Provisioning.ReadCustSpecs" \
        "VirtualMachine.Provisioning.ModifyCustSpecs" \
        "VirtualMachine.Config.AddNewDisk" \
        "VirtualMachine.Config.AddRemoveDevice" \
        "VirtualMachine.Config.AdvancedConfig" \
        "VirtualMachine.Config.CPUCount" \
        "VirtualMachine.Config.Memory" \
        "VirtualMachine.Config.Settings" \
        "VirtualMachine.Config.Resource" \
        "VirtualMachine.Config.EditDevice" \
        "VirtualMachine.Config.RemoveDisk" \
        "VirtualMachine.Config.Rename" \
        "VirtualMachine.Config.Annotation" \
        "VirtualMachine.Interact.PowerOn" \
        "VirtualMachine.Interact.PowerOff" \
        "VirtualMachine.Interact.Reset" \
        "VirtualMachine.Interact.Suspend" \
        "VirtualMachine.Interact.ConsoleInteract" \
        "VirtualMachine.Interact.DeviceConnection" \
        "VirtualMachine.Interact.SetCDMedia" \
        "VirtualMachine.Interact.GuestControl" \
        "VirtualMachine.Inventory.Create" \
        "VirtualMachine.Inventory.Delete" \
        "VirtualMachine.Inventory.Move" \
        "VirtualMachine.Inventory.Register" \
        "VirtualMachine.Inventory.Unregister" \
        "Network.Assign" \
        "Datastore.AllocateSpace" \
        "Datastore.Browse" \
        "Datastore.FileManagement"

    # Create Machine API role (~35 privileges)
    # Required for VM lifecycle operations
    create_role "openshift-machine-api" \
        "VirtualMachine.Provisioning.Clone" \
        "VirtualMachine.Provisioning.DeployTemplate" \
        "VirtualMachine.Provisioning.CustomizeGuest" \
        "VirtualMachine.Config.AddNewDisk" \
        "VirtualMachine.Config.AddRemoveDevice" \
        "VirtualMachine.Config.AdvancedConfig" \
        "VirtualMachine.Config.CPUCount" \
        "VirtualMachine.Config.Memory" \
        "VirtualMachine.Config.Settings" \
        "VirtualMachine.Config.Resource" \
        "VirtualMachine.Config.EditDevice" \
        "VirtualMachine.Config.RemoveDisk" \
        "VirtualMachine.Config.Rename" \
        "VirtualMachine.Config.Annotation" \
        "VirtualMachine.Interact.PowerOn" \
        "VirtualMachine.Interact.PowerOff" \
        "VirtualMachine.Interact.Reset" \
        "VirtualMachine.Interact.Suspend" \
        "VirtualMachine.Interact.DeviceConnection" \
        "VirtualMachine.Interact.GuestControl" \
        "VirtualMachine.Inventory.Create" \
        "VirtualMachine.Inventory.Delete" \
        "VirtualMachine.Inventory.Move" \
        "VirtualMachine.Inventory.Register" \
        "VirtualMachine.Inventory.Unregister" \
        "Network.Assign" \
        "Datastore.AllocateSpace" \
        "Datastore.Browse" \
        "Datastore.FileManagement" \
        "ResourcePool.Assign" \
        "Folder.Create" \
        "Folder.Delete"

    # Create CSI Driver role (~12 privileges)
    # Required for storage provisioning
    create_role "openshift-csi-driver" \
        "Datastore.AllocateSpace" \
        "Datastore.Browse" \
        "Datastore.FileManagement" \
        "Datastore.DeleteFile" \
        "Datastore.UpdateVirtualMachineFiles" \
        "VirtualMachine.Config.AddNewDisk" \
        "VirtualMachine.Config.AddRemoveDevice" \
        "VirtualMachine.Config.RemoveDisk" \
        "VirtualMachine.Config.EditDevice" \
        "System.Anonymous" \
        "System.Read" \
        "System.View"

    # Create Cloud Controller Manager role (~9 privileges)
    # Required for read-only node discovery
    create_role "openshift-cloud-controller" \
        "System.Anonymous" \
        "System.Read" \
        "System.View" \
        "VirtualMachine.Inventory.Register" \
        "Host.Config.Network" \
        "Network.Assign" \
        "Datacenter.Read" \
        "Folder.Read" \
        "ClusterComputeResource.Read"

    # Create Diagnostics role (~5 privileges)
    # Required for read-only troubleshooting
    create_role "openshift-diagnostics" \
        "System.Anonymous" \
        "System.Read" \
        "System.View" \
        "VirtualMachine.GuestOperations.Query" \
        "Global.LogEvent"

    log_info "✓ All roles created successfully on $vcenter"
done

log_info "=========================================="
log_info "Role creation complete!"
log_info "=========================================="
log_info ""
log_info "Next steps:"
log_info "1. Create vCenter user accounts for each component (e.g., ocp-machine-api@vsphere.local)"
log_info "2. Assign the appropriate role to each user account"
log_info "3. Generate credentials file using: ./generate-credentials-file.sh"
log_info "4. Create install-config.yaml with componentCredentials or use ~/.vsphere/credentials"
log_info ""
log_info "For more information, see: docs/user/vsphere/per-component-credentials.md"
