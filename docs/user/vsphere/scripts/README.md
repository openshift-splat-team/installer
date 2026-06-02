# vSphere Per-Component Credentials - Automation Scripts

This directory contains automation scripts to help vSphere administrators set up per-component credentials for OpenShift clusters.

## Scripts

### 1. create-component-roles.sh (Linux/macOS)

Creates five vCenter roles using the `govc` CLI tool.

**Prerequisites:**
- govc installed: https://github.com/vmware/govmomi/tree/main/govc
- vCenter administrator credentials

**Usage:**
```bash
# Single vCenter
./create-component-roles.sh vcenter.example.com administrator@vsphere.local 'password'

# Multiple vCenters
export VCENTERS="vcenter1.example.com vcenter2.example.com"
export VCENTER_USER="administrator@vsphere.local"
export VCENTER_PASSWORD="password"
./create-component-roles.sh
```

**Roles Created:**
- `openshift-installer` (~45 privileges)
- `openshift-machine-api` (~35 privileges)
- `openshift-csi-driver` (~12 privileges)
- `openshift-cloud-controller` (~9 privileges)
- `openshift-diagnostics` (~5 privileges)

### 2. create-component-roles.ps1 (Windows)

PowerCLI version of the role creation script for Windows administrators.

**Prerequisites:**
- VMware PowerCLI: `Install-Module -Name VMware.PowerCLI -Scope CurrentUser`
- vCenter administrator credentials

**Usage:**
```powershell
# Single vCenter
.\create-component-roles.ps1 -VCenter "vcenter.example.com" -Username "administrator@vsphere.local" -Password "password"

# Multiple vCenters
$vCenters = @("vcenter1.example.com", "vcenter2.example.com")
.\create-component-roles.ps1 -VCenters $vCenters -Username "administrator@vsphere.local" -Password "password"
```

### 3. generate-credentials-file.sh

Generates a template YAML credentials file with proper permissions.

**Prerequisites:**
- None (generates template only)

**Usage:**
```bash
# Single vCenter (default)
./generate-credentials-file.sh

# Single vCenter (custom)
./generate-credentials-file.sh vcenter.example.com

# Multiple vCenters
./generate-credentials-file.sh vcenter1.example.com vcenter2.example.com
```

**Output:**
- File: `~/.vsphere/credentials`
- Permissions: `0600` (read/write owner only)
- Format: INI-style sections with component credentials

## Workflow

### Quick Start

```bash
# 1. Create vCenter roles
./create-component-roles.sh vcenter.example.com admin@vsphere.local 'password'

# 2. Create vCenter user accounts (manual, in vCenter UI)
#    - ocp-installer@vsphere.local
#    - ocp-machine-api@vsphere.local
#    - ocp-csi-driver@vsphere.local
#    - ocp-cloud-controller@vsphere.local
#    - ocp-diagnostics@vsphere.local

# 3. Assign roles to users (manual, in vCenter UI)
#    Administration > Access Control > Global Permissions

# 4. Generate credentials file
./generate-credentials-file.sh vcenter.example.com

# 5. Edit credentials file, replace <PLACEHOLDER> values
vi ~/.vsphere/credentials

# 6. Install cluster
openshift-install create cluster --dir <install-dir>
```

### Detailed Guide

See [../per-component-credentials.md](../per-component-credentials.md) for complete step-by-step instructions.

## Security Notes

- **Credentials File:** Must have `0600` permissions (enforced by installer)
- **Passwords:** Use strong, random passwords (20+ characters)
- **Storage:** Store credentials in enterprise password vault
- **Version Control:** NEVER commit credentials file to git
- **Post-Install:** Disable installer account after installation completes

## Troubleshooting

### govc: command not found

Install govc:
```bash
# macOS
brew install govmomi/tap/govc

# Linux
curl -L -o - "https://github.com/vmware/govmomi/releases/latest/download/govc_$(uname -s)_$(uname -m).tar.gz" | tar -C /usr/local/bin -xvzf - govc
```

### PowerCLI not found

Install PowerCLI:
```powershell
Install-Module -Name VMware.PowerCLI -Scope CurrentUser
```

### Role already exists

Scripts will update existing roles if they already exist. To force recreation:
```bash
# govc
govc role.remove openshift-installer
./create-component-roles.sh ...

# PowerCLI
Remove-VIRole -Name "openshift-installer" -Confirm:$false
.\create-component-roles.ps1 ...
```

### Invalid certificate errors

For self-signed certificates:
```bash
# govc
export GOVC_INSECURE=1

# PowerCLI (already set in script)
Set-PowerCLIConfiguration -InvalidCertificateAction Ignore -Confirm:$false
```

## Support

For issues, questions, or feature requests:
- Documentation: [../per-component-credentials.md](../per-component-credentials.md)
- OpenShift Installation Guide: [../install.md](../install.md)
- Privileges Reference: [../privileges.md](../privileges.md)
