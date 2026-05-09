#!/usr/bin/env bash
# create-roles.sh — create per-component vCenter roles for OpenShift installation.
#
# Requirements:
#   govc (https://github.com/vmware/govmomi/tree/main/govc)
#   GOVC_URL, GOVC_USERNAME, GOVC_PASSWORD environment variables set.
#
# Usage:
#   export GOVC_URL=vcenter.example.com
#   export GOVC_USERNAME=administrator@vsphere.local
#   export GOVC_PASSWORD=secret
#   bash create-roles.sh

set -euo pipefail

# --------------------------------------------------------------------------
# openshift-vsphere-machineapi — 19 privileges required by the Machine API
# --------------------------------------------------------------------------
govc role.create openshift-vsphere-machineapi \
  Datastore.AllocateSpace \
  Network.Assign \
  Resource.AssignVMToPool \
  VirtualMachine.Config.AddExistingDisk \
  VirtualMachine.Config.AddNewDisk \
  VirtualMachine.Config.AddRemoveDevice \
  VirtualMachine.Config.AdvancedConfig \
  VirtualMachine.Config.CPUCount \
  VirtualMachine.Config.DiskExtend \
  VirtualMachine.Config.EditDevice \
  VirtualMachine.Config.Memory \
  VirtualMachine.Config.RemoveDisk \
  VirtualMachine.Config.Resource \
  VirtualMachine.Config.Settings \
  VirtualMachine.Interact.PowerOff \
  VirtualMachine.Interact.PowerOn \
  VirtualMachine.Interact.Reset \
  VirtualMachine.Inventory.Create \
  VirtualMachine.Inventory.Delete

echo "Created role: openshift-vsphere-machineapi (19 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-csidriver — 6 privileges required by the CSI driver
# --------------------------------------------------------------------------
govc role.create openshift-vsphere-csidriver \
  Datastore.AllocateSpace \
  Datastore.FileManagement \
  StoragePod.Config \
  VirtualMachine.Config.AddExistingDisk \
  VirtualMachine.Config.AddNewDisk \
  VirtualMachine.Config.RemoveDisk

echo "Created role: openshift-vsphere-csidriver (6 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-cloudcontroller — 3 privileges required by Cloud Controller Manager
# --------------------------------------------------------------------------
govc role.create openshift-vsphere-cloudcontroller \
  System.Read \
  System.View \
  VirtualMachine.Inventory.Create

echo "Created role: openshift-vsphere-cloudcontroller (3 privileges)"

# --------------------------------------------------------------------------
# openshift-vsphere-diagnostics — 2 privileges required by the vSphere Problem Detector
# --------------------------------------------------------------------------
govc role.create openshift-vsphere-diagnostics \
  Sessions.ValidateSession \
  StorageProfile.View

echo "Created role: openshift-vsphere-diagnostics (2 privileges)"

echo ""
echo "All four per-component vCenter roles created successfully."
echo "Assign each role to the corresponding service account on the appropriate vCenter objects."
