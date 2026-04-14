# create-component-roles.ps1
# Creates vCenter roles for OpenShift per-component credentials using PowerCLI
#
# Usage:
#   Single vCenter:
#     .\create-component-roles.ps1 -VCenter "vcenter.example.com" -Username "administrator@vsphere.local" -Password "password"
#
#   Multiple vCenters:
#     $vCenters = @("vcenter1.example.com", "vcenter2.example.com")
#     .\create-component-roles.ps1 -VCenters $vCenters -Username "administrator@vsphere.local" -Password "password"
#
# Prerequisites:
#   - VMware PowerCLI installed (Install-Module -Name VMware.PowerCLI -Scope CurrentUser)
#   - vCenter administrator credentials
#   - Network connectivity to vCenter servers
#
# This script creates five vCenter roles with precisely scoped privileges for OpenShift components:
#   - openshift-installer: Installation and infrastructure provisioning (~45 privileges)
#   - openshift-machine-api: VM lifecycle management (~35 privileges)
#   - openshift-csi-driver: Storage provisioning (~12 privileges)
#   - openshift-cloud-controller: Read-only node discovery (~9 privileges)
#   - openshift-diagnostics: Read-only troubleshooting (~5 privileges)

param(
    [Parameter(Mandatory=$false)]
    [string]$VCenter,

    [Parameter(Mandatory=$false)]
    [string[]]$VCenters,

    [Parameter(Mandatory=$true)]
    [string]$Username,

    [Parameter(Mandatory=$true)]
    [string]$Password
)

# Set PowerCLI configuration to ignore invalid certificates (optional)
Set-PowerCLIConfiguration -InvalidCertificateAction Ignore -Confirm:$false -Scope Session | Out-Null
Set-PowerCLIConfiguration -ParticipateInCEIP $false -Confirm:$false -Scope Session | Out-Null

# Function to write colored output
function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Check if PowerCLI is installed
if (-not (Get-Module -ListAvailable -Name VMware.PowerCLI)) {
    Write-Error-Custom "VMware PowerCLI is not installed"
    Write-Info "Install PowerCLI: Install-Module -Name VMware.PowerCLI -Scope CurrentUser"
    exit 1
}

# Import PowerCLI module
Import-Module VMware.PowerCLI -ErrorAction Stop

# Determine vCenter list
if ($VCenter) {
    $vCenterList = @($VCenter)
} elseif ($VCenters) {
    $vCenterList = $VCenters
} else {
    Write-Error-Custom "Must specify either -VCenter or -VCenters parameter"
    exit 1
}

# Function to create or update a role
function New-OrUpdate-VIRole {
    param(
        [string]$Name,
        [string[]]$Privileges
    )

    Write-Info "Creating role: $Name ($($Privileges.Count) privileges)"

    try {
        # Check if role exists
        $existingRole = Get-VIRole -Name $Name -ErrorAction SilentlyContinue

        if ($existingRole) {
            Write-Warn "Role '$Name' already exists, updating privileges"
            Set-VIRole -Role $existingRole -AddPrivilege (Get-VIPrivilege -Id $Privileges) -ErrorAction Stop | Out-Null
        } else {
            New-VIRole -Name $Name -Privilege (Get-VIPrivilege -Id $Privileges) -ErrorAction Stop | Out-Null
        }

        Write-Info "✓ Role '$Name' created/updated successfully"
        return $true
    } catch {
        Write-Error-Custom "✗ Failed to create role '$Name': $_"
        return $false
    }
}

# Main execution loop for each vCenter
foreach ($vCenterServer in $vCenterList) {
    Write-Info "=========================================="
    Write-Info "Processing vCenter: $vCenterServer"
    Write-Info "=========================================="

    try {
        # Connect to vCenter
        $connection = Connect-VIServer -Server $vCenterServer -User $Username -Password $Password -ErrorAction Stop
        Write-Info "Successfully connected to $vCenterServer"

        # Create Installer role (~45 privileges)
        # Required for cluster infrastructure deployment
        $installerPrivileges = @(
            "Folder.Create",
            "Folder.Delete",
            "Folder.Move",
            "Folder.Rename",
            "ResourcePool.Create",
            "ResourcePool.Delete",
            "ResourcePool.Assign",
            "VirtualMachine.Provisioning.Clone",
            "VirtualMachine.Provisioning.DeployTemplate",
            "VirtualMachine.Provisioning.MarkAsTemplate",
            "VirtualMachine.Provisioning.MarkAsVM",
            "VirtualMachine.Provisioning.CustomizeGuest",
            "VirtualMachine.Provisioning.ReadCustSpecs",
            "VirtualMachine.Provisioning.ModifyCustSpecs",
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
            "VirtualMachine.Interact.PowerOn",
            "VirtualMachine.Interact.PowerOff",
            "VirtualMachine.Interact.Reset",
            "VirtualMachine.Interact.Suspend",
            "VirtualMachine.Interact.ConsoleInteract",
            "VirtualMachine.Interact.DeviceConnection",
            "VirtualMachine.Interact.SetCDMedia",
            "VirtualMachine.Interact.GuestControl",
            "VirtualMachine.Inventory.Create",
            "VirtualMachine.Inventory.Delete",
            "VirtualMachine.Inventory.Move",
            "VirtualMachine.Inventory.Register",
            "VirtualMachine.Inventory.Unregister",
            "Network.Assign",
            "Datastore.AllocateSpace",
            "Datastore.Browse",
            "Datastore.FileManagement"
        )
        New-OrUpdate-VIRole -Name "openshift-installer" -Privileges $installerPrivileges

        # Create Machine API role (~35 privileges)
        # Required for VM lifecycle operations
        $machineAPIPrivileges = @(
            "VirtualMachine.Provisioning.Clone",
            "VirtualMachine.Provisioning.DeployTemplate",
            "VirtualMachine.Provisioning.CustomizeGuest",
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
            "VirtualMachine.Interact.PowerOn",
            "VirtualMachine.Interact.PowerOff",
            "VirtualMachine.Interact.Reset",
            "VirtualMachine.Interact.Suspend",
            "VirtualMachine.Interact.DeviceConnection",
            "VirtualMachine.Interact.GuestControl",
            "VirtualMachine.Inventory.Create",
            "VirtualMachine.Inventory.Delete",
            "VirtualMachine.Inventory.Move",
            "VirtualMachine.Inventory.Register",
            "VirtualMachine.Inventory.Unregister",
            "Network.Assign",
            "Datastore.AllocateSpace",
            "Datastore.Browse",
            "Datastore.FileManagement",
            "ResourcePool.Assign",
            "Folder.Create",
            "Folder.Delete"
        )
        New-OrUpdate-VIRole -Name "openshift-machine-api" -Privileges $machineAPIPrivileges

        # Create CSI Driver role (~12 privileges)
        # Required for storage provisioning
        $csiDriverPrivileges = @(
            "Datastore.AllocateSpace",
            "Datastore.Browse",
            "Datastore.FileManagement",
            "Datastore.DeleteFile",
            "Datastore.UpdateVirtualMachineFiles",
            "VirtualMachine.Config.AddNewDisk",
            "VirtualMachine.Config.AddRemoveDevice",
            "VirtualMachine.Config.RemoveDisk",
            "VirtualMachine.Config.EditDevice",
            "System.Anonymous",
            "System.Read",
            "System.View"
        )
        New-OrUpdate-VIRole -Name "openshift-csi-driver" -Privileges $csiDriverPrivileges

        # Create Cloud Controller Manager role (~9 privileges)
        # Required for read-only node discovery
        $cloudControllerPrivileges = @(
            "System.Anonymous",
            "System.Read",
            "System.View",
            "VirtualMachine.Inventory.Register",
            "Host.Config.Network",
            "Network.Assign",
            "Datacenter.Read",
            "Folder.Read",
            "ClusterComputeResource.Read"
        )
        New-OrUpdate-VIRole -Name "openshift-cloud-controller" -Privileges $cloudControllerPrivileges

        # Create Diagnostics role (~5 privileges)
        # Required for read-only troubleshooting
        $diagnosticsPrivileges = @(
            "System.Anonymous",
            "System.Read",
            "System.View",
            "VirtualMachine.GuestOperations.Query",
            "Global.LogEvent"
        )
        New-OrUpdate-VIRole -Name "openshift-diagnostics" -Privileges $diagnosticsPrivileges

        Write-Info "✓ All roles created successfully on $vCenterServer"

        # Disconnect from vCenter
        Disconnect-VIServer -Server $vCenterServer -Confirm:$false

    } catch {
        Write-Error-Custom "Failed to process vCenter $vCenterServer : $_"
        continue
    }
}

Write-Info "=========================================="
Write-Info "Role creation complete!"
Write-Info "=========================================="
Write-Info ""
Write-Info "Next steps:"
Write-Info "1. Create vCenter user accounts for each component (e.g., ocp-machine-api@vsphere.local)"
Write-Info "2. Assign the appropriate role to each user account"
Write-Info "3. Generate credentials file or configure install-config.yaml with componentCredentials"
Write-Info ""
Write-Info "For more information, see: docs/user/vsphere/per-component-credentials.md"
