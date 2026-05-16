# Per-Component vSphere Credentials

OpenShift supports assigning distinct vCenter credentials to each cluster component, replacing the
single shared account used in the default (passthrough) mode. This reduces blast radius from
credential compromise, enables vCenter audit log attribution by component, and helps meet
separation-of-duties requirements.

## Overview

When per-component credentials are configured, the installer:

1. Validates that each credential has the required privileges before cluster creation begins.
2. Creates a dedicated Kubernetes Secret per component in the `kube-system` namespace.
3. Sets `credentialsMode: PerComponent` in the `Infrastructure` CR so operators know to use
   their component-specific secret instead of the shared `vsphere-cloud-credentials` secret.

If no per-component credentials are provided, the cluster operates in passthrough mode using
the single account from `platform.vsphere.vcenters[].username` — existing behavior is unchanged.

## Required Privileges Per Component

The privilege sets below are enforced by Cloud Credential Operator (CCO) at cluster bootstrap.
Each set is the minimum required; accounts may have additional privileges without issue.

### Machine API Operator (19 privileges)

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

### vSphere CSI Driver (6 privileges)

| Privilege |
|-----------|
| `Datastore.AllocateSpace` |
| `Datastore.FileManagement` |
| `StoragePod.Config` |
| `VirtualMachine.Config.AddExistingDisk` |
| `VirtualMachine.Config.AddNewDisk` |
| `VirtualMachine.Config.RemoveDisk` |

### Cloud Controller Manager (3 privileges)

| Privilege |
|-----------|
| `System.Read` |
| `System.View` |
| `VirtualMachine.Inventory.Create` |

### Diagnostics / vSphere Problem Detector (2 privileges)

| Privilege |
|-----------|
| `Sessions.ValidateSession` |
| `StorageProfile.View` |

## Creating vCenter Roles

The `upi/vsphere/per-component-credentials/` directory in this repository includes automation
scripts to create vCenter roles with the privilege sets above:

- **`create-roles.sh`** — uses the `govc` CLI
- **`create-roles.ps1`** — uses PowerCLI (Windows / cross-platform)

Both scripts create four roles:

| Role Name | Component | Privilege Count |
|-----------|-----------|-----------------|
| `openshift-vsphere-machineapi` | Machine API Operator | 19 |
| `openshift-vsphere-csidriver` | CSI Driver | 6 |
| `openshift-vsphere-cloudcontroller` | Cloud Controller Manager | 3 |
| `openshift-vsphere-diagnostics` | Diagnostics | 2 |

Example using `govc`:

```bash
export GOVC_URL=vcenter1.example.com
export GOVC_USERNAME=administrator@vsphere.local
export GOVC_PASSWORD=password
bash upi/vsphere/per-component-credentials/create-roles.sh
```

After creating roles, assign each role to a dedicated vCenter account and note the credentials
for use in the configuration steps below.

## Configuration

### Method 1: install-config.yaml

Add a `componentCredentials` block inside each vCenter entry:

```yaml
platform:
  vsphere:
    vcenters:
    - server: vcenter1.example.com
      username: installer@vsphere.local
      password: installer-password
      datacenters:
      - datacenter1
      componentCredentials:
        machineAPI:
          username: machine-api@vsphere.local
          password: machine-api-password
        csiDriver:
          username: csi@vsphere.local
          password: csi-password
        cloudController:
          username: cloud-controller@vsphere.local
          password: cloud-controller-password
        diagnostics:
          username: diagnostics@vsphere.local
          password: diagnostics-password
```

**Multi-vCenter example** — specify `componentCredentials` under each vCenter:

```yaml
platform:
  vsphere:
    vcenters:
    - server: vcenter1.example.com
      username: installer@vsphere.local
      password: installer-password
      componentCredentials:
        machineAPI:
          username: machine-api@vc1
          password: password1
        csiDriver:
          username: csi@vc1
          password: password1
    - server: vcenter2.example.com
      username: installer@vsphere.local
      password: installer-password
      componentCredentials:
        machineAPI:
          username: machine-api@vc2
          password: password2
        csiDriver:
          username: csi@vc2
          password: password2
```

Only the components you specify are validated. Components without credentials fall back to the
shared account at runtime.

### Method 2: ~/.vsphere/credentials File

Use the INI-style credentials file for interactive or CI deployments where embedding credentials
in `install-config.yaml` is undesirable.

```ini
[vcenter1.example.com]
user = installer@vsphere.local
password = installer-password
machine-api.user = machine-api@vsphere.local
machine-api.password = machine-api-password
csi-driver.user = csi@vsphere.local
csi-driver.password = csi-password
cloud-controller.user = cloud-controller@vsphere.local
cloud-controller.password = cloud-controller-password
diagnostics.user = diagnostics@vsphere.local
diagnostics.password = diagnostics-password

[vcenter2.example.com]
user = installer@vsphere.local
password = installer-password
machine-api.user = machine-api@vc2
machine-api.password = vc2-password
```

**File permissions requirement:** The installer refuses to proceed if `~/.vsphere/credentials`
has permissions more open than `0600`. Set correct permissions before running `openshift-install`:

```bash
chmod 0600 ~/.vsphere/credentials
```

The credentials file can be generated from a template using the helper script in
`upi/vsphere/per-component-credentials/generate-credentials.sh`.

## How Components Use Credentials

After installation, each component reads from its dedicated Secret in `kube-system`:

| Component | Secret Name |
|-----------|-------------|
| Machine API Operator | `vsphere-machine-api-creds` |
| vSphere CSI Driver | `vsphere-storage-creds` |
| Cloud Controller Manager | `vsphere-cloud-controller-creds` |
| Diagnostics / Problem Detector | `vsphere-problem-detector-creds` |

Secrets use per-vCenter key naming:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-machine-api-creds
  namespace: kube-system
type: Opaque
data:
  vcenter1.example.com.username: <base64>
  vcenter1.example.com.password: <base64>
  vcenter2.example.com.username: <base64>
  vcenter2.example.com.password: <base64>
```

If a component's Secret is absent or empty, CCO falls back to the shared
`vsphere-cloud-credentials` secret and logs a warning.

## Credential Validation

The installer calls `openstack-vsphere` pre-flight validation before cluster creation. For each
configured component credential it:

1. Connects to the vCenter as the component account.
2. Calls `AuthorizationManager.FetchUserPrivilegeOnEntities` to list the account's effective
   privileges.
3. Compares against the required privilege set for that component.
4. Blocks installation and reports missing privileges if validation fails.

Example error output when a privilege is missing:

```text
FATAL: failed to create cluster: Credential validation failed for machineAPI on vcenter1.example.com: missing privileges: [VirtualMachine.Inventory.Create]
```

## Migrating an Existing Cluster

Existing clusters using a single shared account can migrate to per-component credentials without
downtime:

1. **Create vCenter accounts** for each component using the scripts in
   `upi/vsphere/per-component-credentials/` or through the vSphere UI. Assign the appropriate
   role from the privilege table above.

2. **Create component Secrets** in `kube-system` with the new credentials:

   ```bash
   kubectl create secret generic vsphere-machine-api-creds \
     --namespace kube-system \
     --from-literal=vcenter1.example.com.username='machine-api@vsphere.local' \
     --from-literal=vcenter1.example.com.password='machine-api-password'
   ```

   Repeat for each component secret.

3. **Verify secrets exist** for all cluster vCenters before proceeding.

4. **Annotate CredentialsRequests** to enable per-component routing. CCO picks up the annotation
   `cloudcredential.openshift.io/vsphere-component` and routes to the component secret:

   ```bash
   # Verify CCO is routing to the component secret (check operator logs)
   kubectl logs -n openshift-cloud-credential-operator \
     deploy/cloud-credential-operator | grep "vsphere-machine-api-creds"
   ```

5. **Validate component health** — all cluster operators should remain `Available=True` and
   `Degraded=False`:

   ```bash
   kubectl get co
   ```

6. **Remove global privileges** from the shared account after confirming all components operate
   correctly with their component credentials.

## Passthrough Mode (Default)

If no `componentCredentials` block is present, the cluster operates in passthrough mode:

- All components use the shared `vsphere-cloud-credentials` secret.
- `credentialsMode` is not set to `PerComponent`.
- No pre-flight per-component validation is performed.
- Existing single-account deployments are unaffected.

## Troubleshooting

For common issues including authentication failures, missing privileges, credential file
permission errors, and vSphere role assignment problems, see
[per-component-credentials-troubleshooting.md](../../dev/vsphere/per-component-credentials-troubleshooting.md)
or the standalone guide at `docs/user/vsphere/per-component-credentials-troubleshooting.md`.

### Quick Reference

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| `Credential validation failed for machineAPI on <vcenter>: missing privileges: [...]` | Account missing listed privileges | Add privileges to the vCenter role assigned to the account |
| `Authentication failed for '<user>' on vCenter '<vcenter>'` | Wrong password or account locked | Verify credentials; check vCenter login events |
| `~/.vsphere/credentials: permissions too open` | File mode is not `0600` | `chmod 0600 ~/.vsphere/credentials` |
| Component operator `Degraded=True` after migration | Secret missing for a vCenter | Verify Secret has keys for every cluster vCenter FQDN |
| CCO warning: `Component 'machineAPI' using fallback shared credentials` | Component secret is empty or absent | Create the component Secret or populate missing keys |
