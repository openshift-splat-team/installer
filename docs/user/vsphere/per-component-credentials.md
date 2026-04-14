# vSphere Per-Component Credentials

## Overview

The vSphere per-component credentials feature enables OpenShift clusters to use distinct vCenter accounts for different cluster components. Instead of using a single high-privilege account for all operations, each component (Machine API, CSI Driver, Cloud Controller Manager, Diagnostics) receives only the vCenter permissions it needs.

### Benefits

- **Reduced Security Blast Radius**: A compromised component cannot access other components' vCenter operations
- **Principle of Least Privilege**: Each component has minimal required permissions, meeting compliance requirements
- **Improved Auditability**: vCenter audit logs distinguish actions by component via separate usernames
- **Independent Credential Rotation**: Update credentials for individual components without cluster downtime
- **Compliance Support**: Meets enterprise IAM requirements for separation of duties

### User Personas

- **Security Administrators**: Require strict IAM policies and blast radius reduction
- **Cloud Architects**: Need to migrate brownfield clusters to meet hardened security standards  
- **vSphere Infrastructure Teams**: Manage vCenter accounts through established provisioning processes

## Architecture

### Component Privilege Requirements

| Component | Privilege Count | Key Operations | Account Scope |
|-----------|-----------------|----------------|---------------|
| **Installer** | ~45 | Full deployment operations: create folders, resource pools, VMs, networks | Installation only (can be disabled post-install) |
| **Machine API** | ~35 | VM lifecycle: create, delete, configure, power operations | Runtime (required for cluster scaling) |
| **CSI Driver** | ~12 | Storage provisioning: create/delete disks, manage datastore files | Runtime (required for PV provisioning) |
| **Cloud Controller Manager** | ~9 | Read-only node discovery and metadata | Runtime (required for node management) |
| **Diagnostics** | ~5 | Read-only troubleshooting and log access | Runtime (optional, used for must-gather) |

### Workflow

```
Administrator                 vCenter                    OpenShift Installer           OpenShift Cluster
     |                           |                              |                              |
     |--1. Create Roles--------->|                              |                              |
     |   (5 component roles)     |                              |                              |
     |                           |                              |                              |
     |--2. Create Users--------->|                              |                              |
     |   (5 component accounts)  |                              |                              |
     |                           |                              |                              |
     |--3. Assign Roles--------->|                              |                              |
     |                           |                              |                              |
     |--4. Generate Creds------->|                              |                              |
     |   (credentials file)      |                              |                              |
     |                           |                              |                              |
     |--5. Run Installer--------------------------------->|                              |
     |                           |                              |                              |
     |                           |<--6. Validate Privileges-----|                              |
     |                           |                              |                              |
     |                           |<--7. Deploy Infrastructure---|                              |
     |                           |   (uses installer account)   |                              |
     |                           |                              |                              |
     |                           |                              |--8. Create Secrets---------->|
     |                           |                              |   (component-specific)       |
     |                           |                              |                              |
     |                           |<--9. Machine API Operations---------------(machine-api account)
     |                           |<--10. CSI Operations----------------------(csi-driver account)
     |                           |<--11. CCM Operations----------------------(cloud-controller account)
     |                           |<--12. Diagnostics--------------------------(diagnostics account)
```

## Prerequisites

### Tools Required

**For Linux/macOS Administrators:**
- `govc` CLI tool ([installation guide](https://github.com/vmware/govmomi/tree/main/govc))
- vCenter administrator credentials
- Network connectivity to vCenter

**For Windows Administrators:**
- VMware PowerCLI (`Install-Module -Name VMware.PowerCLI -Scope CurrentUser`)
- vCenter administrator credentials
- Network connectivity to vCenter

### vCenter Requirements

- vCenter 6.7 or later
- Administrator access to create roles and users
- Sufficient vCenter licenses for user accounts

## Step-by-Step Guide

### Step 1: Create vCenter Roles

Use the provided automation scripts to create five vCenter roles with precisely scoped privileges.

#### Linux/macOS (using govc)

```bash
cd docs/user/vsphere/scripts/

# Single vCenter
./create-component-roles.sh vcenter.example.com administrator@vsphere.local 'password'

# Multiple vCenters
export VCENTERS="vcenter1.example.com vcenter2.example.com"
export VCENTER_USER="administrator@vsphere.local"
export VCENTER_PASSWORD="password"
./create-component-roles.sh
```

#### Windows (using PowerCLI)

```powershell
cd docs\user\vsphere\scripts\

# Single vCenter
.\create-component-roles.ps1 -VCenter "vcenter.example.com" -Username "administrator@vsphere.local" -Password "password"

# Multiple vCenters
$vCenters = @("vcenter1.example.com", "vcenter2.example.com")
.\create-component-roles.ps1 -VCenters $vCenters -Username "administrator@vsphere.local" -Password "password"
```

**Roles Created:**
- `openshift-installer` (~45 privileges)
- `openshift-machine-api` (~35 privileges)
- `openshift-csi-driver` (~12 privileges)
- `openshift-cloud-controller` (~9 privileges)
- `openshift-diagnostics` (~5 privileges)

### Step 2: Create vCenter User Accounts

Create five user accounts in vCenter, one for each component. Use descriptive usernames for clear audit trails.

**Recommended Naming Convention:**
- `ocp-installer@vsphere.local` → Installer role
- `ocp-machine-api@vsphere.local` → Machine API role
- `ocp-csi-driver@vsphere.local` → CSI Driver role
- `ocp-cloud-controller@vsphere.local` → Cloud Controller Manager role
- `ocp-diagnostics@vsphere.local` → Diagnostics role

**Password Requirements:**
- Minimum 20 characters
- Random, generated passwords
- Store in enterprise password vault

### Step 3: Assign Roles to Users

In vCenter, assign the appropriate role to each user account:

1. Navigate to **Administration > Access Control > Global Permissions**
2. Click **+** to add permission
3. Select user (e.g., `ocp-machine-api@vsphere.local`)
4. Select role (e.g., `openshift-machine-api`)
5. Check **Propagate to children**
6. Click **OK**
7. Repeat for all five component accounts

**Alternative:** Assign roles at Datacenter or Cluster level for more granular control.

### Step 4: Generate Credentials File

Use the provided script to generate a template credentials file:

```bash
cd docs/user/vsphere/scripts/

# Single vCenter
./generate-credentials-file.sh vcenter.example.com

# Multiple vCenters
./generate-credentials-file.sh vcenter1.example.com vcenter2.example.com
```

**Output:** `~/.vsphere/credentials` with 0600 permissions

### Step 5: Populate Credentials File

Edit `~/.vsphere/credentials` and replace all `<PLACEHOLDER>` values with actual credentials:

```ini
[vcenter.example.com]
user = ocp-installer@vsphere.local
password = <actual-installer-password>
machine-api.user = ocp-machine-api@vsphere.local
machine-api.password = <actual-machine-api-password>
csi-driver.user = ocp-csi-driver@vsphere.local
csi-driver.password = <actual-csi-driver-password>
cloud-controller.user = ocp-cloud-controller@vsphere.local
cloud-controller.password = <actual-cloud-controller-password>
diagnostics.user = ocp-diagnostics@vsphere.local
diagnostics.password = <actual-diagnostics-password>
```

**Verify Permissions:**
```bash
ls -la ~/.vsphere/credentials
# Should show: -rw------- (0600)
```

### Step 6: Create Cluster

#### Option A: Using Credentials File

If credentials are in `~/.vsphere/credentials`, the installer will automatically read them:

```bash
openshift-install create cluster --dir <install-dir>
```

The installer will:
1. Read credentials from `~/.vsphere/credentials`
2. Validate each component's privileges using vSphere AuthorizationManager API
3. Deploy infrastructure using installer account
4. Create component-specific secrets in appropriate namespaces
5. Components will authenticate with their respective accounts

#### Option B: Using install-config.yaml

Alternatively, embed credentials in `install-config.yaml`:

```yaml
apiVersion: v1
baseDomain: example.com
metadata:
  name: vsphere-per-component
platform:
  vsphere:
    vCenter: vcenter.example.com
    datacenter: DC1
    defaultDatastore: datastore1
    componentCredentials:
      installer:
        username: ocp-installer@vsphere.local
        password: <installer-password>
      machineAPI:
        username: ocp-machine-api@vsphere.local
        password: <machine-api-password>
      csiDriver:
        username: ocp-csi-driver@vsphere.local
        password: <csi-password>
      cloudController:
        username: ocp-cloud-controller@vsphere.local
        password: <ccm-password>
      diagnostics:
        username: ocp-diagnostics@vsphere.local
        password: <diagnostics-password>
```

**Precedence:** install-config.yaml credentials override `~/.vsphere/credentials`

## Multi-vCenter Support

For deployments spanning multiple vCenter servers:

### Credentials File Format

```ini
[vcenter1.example.com]
user = ocp-installer@vsphere.local
password = <password>
machine-api.user = ocp-machine-api@vsphere.local
machine-api.password = <password>
csi-driver.user = ocp-csi-driver@vsphere.local
csi-driver.password = <password>

[vcenter2.example.com]
user = ocp-installer@vsphere.local
password = <password>
machine-api.user = ocp-machine-api@vsphere.local
machine-api.password = <password>
```

### install-config.yaml Format

```yaml
platform:
  vsphere:
    vCenter: vcenter1.example.com  # Default vCenter
    componentCredentials:
      installer:
        username: ocp-installer@vsphere.local
        password: <password>
      machineAPI:
        username: ocp-machine-api@vsphere.local
        password: <password>
        vCenter: vcenter1.example.com
      csiDriver:
        username: ocp-csi-driver@vsphere.local
        password: <password>
        vCenter: vcenter2.example.com  # CSI uses different vCenter
```

**Secrets:** Multi-vCenter secrets use FQDN-keyed format (`vcenter1.example.com.username`)

## Brownfield Migration

Migrate existing clusters from single-account (passthrough) mode to per-component credentials.

### Prerequisites

- Existing vSphere IPI cluster (installed with single account)
- Per-component vCenter accounts created and assigned roles
- Credentials file with per-component credentials

### Migration Command

```bash
openshift-install vsphere migrate-to-per-component \
  --kubeconfig=/path/to/kubeconfig \
  --credentials-file=~/.vsphere/credentials \
  --validate-privileges
```

### Migration Steps

The migration tool will:
1. Read per-component credentials from credentials file
2. Validate each component's privileges using vSphere API
3. Create backup of original passthrough-mode secret
4. Create component-specific secrets in appropriate namespaces
5. Update CCO configuration to enable per-component mode
6. Restart component operators (Machine API, CSI, CCM)
7. Verify each component reconnects successfully
8. Log migration completion

### Rollback

If migration fails, the tool automatically:
1. Restores original passthrough secret from backup
2. Deletes component-specific secrets
3. Reverts CCO configuration
4. Logs detailed error messages

**Manual Rollback** (if automatic rollback fails):
```bash
# Restore original secret
oc get secret vsphere-cloud-credentials-backup -n kube-system -o yaml | \
  sed 's/vsphere-cloud-credentials-backup/vsphere-cloud-credentials/' | \
  oc apply -f -

# Restart operators
oc delete pod -n openshift-machine-api -l api=clusterapi
oc delete pod -n openshift-cluster-csi-drivers -l app=vsphere-csi-driver
oc delete pod -n openshift-cloud-controller-manager -l app=vsphere-cloud-controller-manager
```

## Security Considerations

### Credential Storage

**Risk:** Component credentials in cluster could be compromised

**Mitigation:**
- Each component's credentials stored in separate secrets in component-specific namespaces
- Kubernetes RBAC limits access to component secrets
- Components can only access their own credentials
- Enable cluster encryption-at-rest for additional protection

### Installer Account Management

**Recommendation:** Disable installer account after installation completes

```bash
# Installation complete
# In vCenter: Administration > Single Sign On > Users and Groups
# Select: ocp-installer@vsphere.local
# Click: Disable Account
```

Installer account is only needed during installation and cluster upgrades.

### Credential Rotation

Rotate component credentials independently:

```bash
# Example: Rotate Machine API credentials

# 1. Update vCenter password for ocp-machine-api@vsphere.local

# 2. Update cluster secret
oc create secret generic machine-api-vsphere-credentials \
  --from-literal=username=ocp-machine-api@vsphere.local \
  --from-literal=password=<new-password> \
  --dry-run=client -o yaml | \
  oc apply -n openshift-machine-api -f -

# 3. Restart Machine API operator
oc delete pod -n openshift-machine-api -l api=clusterapi

# 4. Verify reconnection
oc logs -n openshift-machine-api -l api=clusterapi | grep "Successfully connected to vCenter"
```

### Audit Trail

Each component uses a distinct vCenter username, enabling clear audit trails:

**vCenter Event Log:**
```
[2026-04-14 10:30:15] ocp-machine-api@vsphere.local: Created VM ocp-worker-3
[2026-04-14 10:32:20] ocp-csi-driver@vsphere.local: Created disk pvc-abc123
[2026-04-14 10:35:10] ocp-cloud-controller@vsphere.local: Read VM metadata for node worker-1
```

Query vCenter events by username to track component-specific actions.

## Troubleshooting

### Privilege Validation Failures

**Error:** `Component machine-api missing required privilege: VirtualMachine.Provisioning.Clone on Datacenter`

**Resolution:**
1. Verify role assignment in vCenter
2. Ensure role includes required privilege
3. Check propagation is enabled (for datacenter/cluster-level assignments)
4. Re-run installer or migration command

### Component Authentication Failures

**Error:** `Component csi-driver failed to authenticate with vCenter vcenter.example.com`

**Resolution:**
1. Verify username and password in credentials file or secret
2. Test authentication using govc:
   ```bash
   export GOVC_URL=vcenter.example.com
   export GOVC_USERNAME=ocp-csi-driver@vsphere.local
   export GOVC_PASSWORD=<password>
   govc about
   ```
3. Check vCenter account is not locked/disabled
4. Verify network connectivity from cluster to vCenter

### Component Fails to Reconnect After Migration

**Error:** `Component machine-api failed to reconnect with new credentials`

**Resolution:**
1. Check component operator logs:
   ```bash
   oc logs -n openshift-machine-api -l api=clusterapi
   ```
2. Verify component secret exists and contains correct credentials:
   ```bash
   oc get secret machine-api-vsphere-credentials -n openshift-machine-api -o yaml
   ```
3. If incorrect, recreate secret and restart operator
4. If persistent, rollback migration and investigate privilege/network issues

### Multi-vCenter Credential Issues

**Error:** `Component csi-driver references vCenter vcenter2.example.com but no credentials provided`

**Resolution:**
1. Ensure credentials file contains section for all referenced vCenters
2. Verify vCenter FQDN matches exactly (case-sensitive)
3. Check component-specific vCenter override in install-config.yaml

### File Permission Errors

**Error:** `Credentials file ~/.vsphere/credentials has permissions 0644, must be 0600`

**Resolution:**
```bash
chmod 0600 ~/.vsphere/credentials
```

## Reference

### Component Secret Locations

| Component | Secret Name | Namespace |
|-----------|-------------|-----------|
| Machine API | `machine-api-vsphere-credentials` | `openshift-machine-api` |
| CSI Driver | `vsphere-csi-credentials` | `openshift-cluster-csi-drivers` |
| Cloud Controller Manager | `vsphere-ccm-credentials` | `openshift-cloud-controller-manager` |
| Diagnostics | `vsphere-diagnostics-credentials` | `openshift-config` |

### Complete Privilege Lists

See automation scripts for complete privilege lists:
- `scripts/create-component-roles.sh` (govc, Linux/macOS)
- `scripts/create-component-roles.ps1` (PowerCLI, Windows)

Or view in code:
- `pkg/asset/installconfig/vsphere/privilegevalidator.go`

### Related Documentation

- [vSphere Installation Guide](install.md)
- [vSphere Privileges (Legacy)](privileges.md)
- [vSphere Requirements](requirements.md)
- [OpenShift Enhancement Proposal](https://github.com/openshift/enhancements/pull/XXXX) (TODO: update link)

## FAQ

**Q: Can I use per-component credentials with Assisted Installer?**  
A: Yes, Assisted Installer UI provides fields for per-component credentials.

**Q: What happens if I only provide some component credentials?**  
A: Components without specific credentials fall back to legacy username/password (passthrough mode).

**Q: Can I use the same username for multiple components?**  
A: Yes, but it defeats the auditability benefit. Distinct usernames are recommended.

**Q: Do I need different accounts for multiple clusters?**  
A: No, the same component accounts can be used across multiple clusters. vSphere permissions are scoped to datacenter/folder.

**Q: Can I disable the installer account after installation?**  
A: Yes, recommended. Installer account is only needed during installation and upgrades.

**Q: How do I verify per-component mode is active?**  
A: Check for component-specific secrets in their namespaces:
```bash
oc get secret machine-api-vsphere-credentials -n openshift-machine-api
oc get secret vsphere-csi-credentials -n openshift-cluster-csi-drivers
oc get secret vsphere-ccm-credentials -n openshift-cloud-controller-manager
```

**Q: Can I mix per-component and passthrough mode?**  
A: Yes, partial component credentials are supported. Unpopulated components use passthrough mode.

**Q: How do I update credentials after installation?**  
A: Update the secret and restart the component operator (see Credential Rotation section).
