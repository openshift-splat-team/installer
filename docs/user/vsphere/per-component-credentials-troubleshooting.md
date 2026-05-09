# Troubleshooting: Per-Component vCenter Credentials

This guide covers the four error scenarios you may encounter when using per-component
vCenter credentials during OpenShift installation.

---

## 1. Missing Privilege

**Error example:**

```
Credential validation failed for machineAPI on vcenter1.example.com: missing privileges: [VirtualMachine.Inventory.Create]
```

The service account assigned to that component is missing one or more required vCenter privileges.

### Remediation

Identify the missing privilege from the error message and grant it to the role using
one of the following methods.

**Using govc:**

```bash
# List current privileges on the role
govc role.ls openshift-vsphere-machineapi

# Add a missing privilege to the role
govc role.update openshift-vsphere-machineapi VirtualMachine.Inventory.Create

# Verify the permission assignment for the user
govc permissions.ls /
```

**Using the vCenter UI:**

1. Navigate to **Administration > Access Control > Roles**.
2. Select the role (e.g. `openshift-vsphere-machineapi`).
3. Click **Edit** and find the missing privilege in the hierarchy.
4. Enable the checkbox and click **Save**.

### Canonical privilege sets per component

The table below lists every required privilege. Use it to cross-check your roles.

#### machineAPI (19 privileges)

| Privilege |
|-----------|
| `Datastore.AllocateSpace` |
| `Network.Assign` |
| `Resource.AssignVMToPool` |
| `VirtualMachine.Config.AddExistingDisk` |
| `VirtualMachine.Config.AddNewDisk` |
| `VirtualMachine.Config.AddRemoveDevice` |
| `VirtualMachine.Config.AdvancedConfig` |
| `VirtualMachine.Config.CPUCount` |
| `VirtualMachine.Config.DiskExtend` |
| `VirtualMachine.Config.EditDevice` |
| `VirtualMachine.Config.Memory` |
| `VirtualMachine.Config.RemoveDisk` |
| `VirtualMachine.Config.Resource` |
| `VirtualMachine.Config.Settings` |
| `VirtualMachine.Interact.PowerOff` |
| `VirtualMachine.Interact.PowerOn` |
| `VirtualMachine.Interact.Reset` |
| `VirtualMachine.Inventory.Create` |
| `VirtualMachine.Inventory.Delete` |

#### csiDriver (6 privileges)

| Privilege |
|-----------|
| `Datastore.AllocateSpace` |
| `Datastore.FileManagement` |
| `StoragePod.Config` |
| `VirtualMachine.Config.AddExistingDisk` |
| `VirtualMachine.Config.AddNewDisk` |
| `VirtualMachine.Config.RemoveDisk` |

#### cloudController (3 privileges)

| Privilege |
|-----------|
| `System.Read` |
| `System.View` |
| `VirtualMachine.Inventory.Create` |

#### diagnostics (2 privileges)

| Privilege |
|-----------|
| `Sessions.ValidateSession` |
| `StorageProfile.View` |

---

## 2. Authentication Failure

**Error example:**

```
Credential validation failed for machineAPI on vcenter1.example.com: authentication error: 535 5.7.8 Error: authentication credentials invalid
```

The username or password for a component service account is incorrect.

### Remediation

Verify the credentials are valid by testing them with `govc`:

```bash
export GOVC_URL=vcenter1.example.com
export GOVC_USERNAME=svc-machineapi@vsphere.local
export GOVC_PASSWORD=<password>

govc about
```

If `govc about` returns an error, the credentials are invalid. Check that:

- `GOVC_USERNAME` matches the account name exactly (including domain suffix, e.g. `@vsphere.local`).
- `GOVC_PASSWORD` does not contain shell metacharacters that could be misinterpreted; quote it.
- The account is not locked out in vCenter (**Administration > Single Sign On > Users and Groups**).

Update `~/.vsphere/credentials` with the corrected values and re-run the installer.

---

## 3. File Permission Error

**Error example:**

```
credentials file ~/.vsphere/credentials has insecure permissions 0644; expected 0600
```

The credentials file must be readable only by the current user to protect passwords.

### Remediation

```bash
chmod 0600 ~/.vsphere/credentials
chmod 0700 ~/.vsphere
```

Verify the result:

```bash
ls -la ~/.vsphere/credentials
# Expected: -rw------- 1 <user> ...
```

If the file was accidentally committed to version control or copied with broad permissions,
rotate all passwords referenced in the file before correcting permissions.

---

## 4. Partial Configuration

**Error example:**

```
install-config.yaml: componentCredentials.machineAPI is set but componentCredentials.csiDriver is missing
```

A partial configuration — where some but not all required components have credentials — is rejected
to prevent runtime failures caused by missing credentials discovered only after installation.

### Remediation

Either provide credentials for **all** components listed in `~/.vsphere/credentials`, or
remove the `componentCredentials` block entirely to fall back to a single shared credential.

Use `generate-credentials.sh` to create a complete template:

```bash
export VSPHERE_HOSTNAMES="vcenter1.example.com"
bash upi/vsphere/per-component-credentials/generate-credentials.sh
```

Fill in all four password fields in the generated file before running the installer.
