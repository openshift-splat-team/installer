#!/usr/bin/pwsh
# create-roles.ps1 — create per-component vCenter roles for OpenShift installation (PowerCLI).
#
# Requirements:
#   VMware PowerCLI module (Install-Module -Name VMware.PowerCLI)
#
# Usage:
#   Connect-VIServer -Server vcenter.example.com -User administrator@vsphere.local -Password secret
#   .\create-roles.ps1

param(
    [string]$Server = $env:GOVC_URL
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# --------------------------------------------------------------------------
# openshift-vsphere-machineapi — 19 privileges required by the Machine API
# --------------------------------------------------------------------------
$machineAPIPrivileges = @(
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
    "VirtualMachine.Inventory.Delete"
)
New-VIRole -Name "openshift-vsphere-machineapi" -Privilege (Get-VIPrivilege -Id $machineAPIPrivileges)
Write-Host "Created role: openshift-vsphere-machineapi (19 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-csidriver — 6 privileges required by the CSI driver
# --------------------------------------------------------------------------
$csiDriverPrivileges = @(
    "Datastore.AllocateSpace",
    "Datastore.FileManagement",
    "StoragePod.Config",
    "VirtualMachine.Config.AddExistingDisk",
    "VirtualMachine.Config.AddNewDisk",
    "VirtualMachine.Config.RemoveDisk"
)
New-VIRole -Name "openshift-vsphere-csidriver" -Privilege (Get-VIPrivilege -Id $csiDriverPrivileges)
Write-Host "Created role: openshift-vsphere-csidriver (6 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-cloudcontroller — 3 privileges required by Cloud Controller Manager
# --------------------------------------------------------------------------
$cloudControllerPrivileges = @(
    "System.Read",
    "System.View",
    "VirtualMachine.Inventory.Create"
)
New-VIRole -Name "openshift-vsphere-cloudcontroller" -Privilege (Get-VIPrivilege -Id $cloudControllerPrivileges)
Write-Host "Created role: openshift-vsphere-cloudcontroller (3 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-diagnostics — 2 privileges required by vSphere Problem Detector
# --------------------------------------------------------------------------
$diagnosticsPrivileges = @(
    "Sessions.ValidateSession",
    "StorageProfile.View"
)
New-VIRole -Name "openshift-vsphere-diagnostics" -Privilege (Get-VIPrivilege -Id $diagnosticsPrivileges)
Write-Host "Created role: openshift-vsphere-diagnostics (2 privileges)"

Write-Host ""
Write-Host "All four per-component vCenter roles created successfully."
Write-Host "Assign each role to the corresponding service account on the appropriate vCenter objects."
