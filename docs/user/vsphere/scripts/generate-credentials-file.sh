#!/bin/bash
# generate-credentials-file.sh
# Generates a template YAML credentials file for OpenShift vSphere per-component credentials
#
# Usage:
#   Single vCenter:
#     ./generate-credentials-file.sh vcenter1.example.com
#
#   Multiple vCenters:
#     ./generate-credentials-file.sh vcenter1.example.com vcenter2.example.com
#
#   Default (generates single vCenter template):
#     ./generate-credentials-file.sh
#
# Prerequisites:
#   - None (this script only generates a template file)
#
# Output:
#   - Creates ~/.vsphere/credentials with 0600 permissions
#   - File contains YAML-formatted credential templates for all components
#
# Security:
#   - File is created with 0600 permissions (read/write for owner only)
#   - Contains placeholder passwords that MUST be replaced before use

set -euo pipefail

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Default credentials file path
CREDENTIALS_FILE="${HOME}/.vsphere/credentials"

# Determine vCenter list
if [ $# -eq 0 ]; then
    VCENTERS=("vcenter.example.com")
    log_info "No vCenters specified, using default template"
else
    VCENTERS=("$@")
    log_info "Generating credentials file for vCenters: ${VCENTERS[*]}"
fi

# Create directory if it doesn't exist
mkdir -p "$(dirname "$CREDENTIALS_FILE")"

# Check if file already exists
if [ -f "$CREDENTIALS_FILE" ]; then
    log_warn "Credentials file already exists at: $CREDENTIALS_FILE"
    read -p "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Exiting without changes"
        exit 0
    fi
    # Backup existing file
    BACKUP_FILE="${CREDENTIALS_FILE}.backup.$(date +%Y%m%d-%H%M%S)"
    cp "$CREDENTIALS_FILE" "$BACKUP_FILE"
    log_info "Backed up existing file to: $BACKUP_FILE"
fi

# Generate YAML credentials file
log_info "Generating credentials file at: $CREDENTIALS_FILE"

cat > "$CREDENTIALS_FILE" << 'YAML_TEMPLATE'
# OpenShift vSphere Per-Component Credentials File
# ================================================
#
# This file contains credentials for OpenShift components to authenticate with vCenter.
# Each component uses a separate account with minimal required privileges.
#
# File format: INI-style sections keyed by vCenter FQDN
# File permissions: MUST be 0600 (read/write for owner only)
#
# Components:
#   - installer: Used during cluster installation only (can be disabled post-install)
#   - machine-api: VM lifecycle management (create, delete, modify VMs)
#   - csi-driver: Storage provisioning (PV creation, disk attachment)
#   - cloud-controller: Read-only node discovery and metadata
#   - diagnostics: Read-only troubleshooting and log access
#
# Security Notes:
#   - Each component should use a DIFFERENT username for auditability
#   - Passwords should be strong (20+ characters, random)
#   - Installer account can be disabled after installation completes
#   - This file should NEVER be committed to version control
#   - After installation, delete or secure this file (chmod 0400 or move to vault)
#
# Prerequisites:
#   1. Create vCenter roles using create-component-roles.sh (govc) or create-component-roles.ps1 (PowerCLI)
#   2. Create vCenter user accounts for each component
#   3. Assign appropriate role to each user account
#   4. Replace <PLACEHOLDER> values below with actual credentials
#
# Usage:
#   - Place this file at ~/.vsphere/credentials
#   - Replace all <PLACEHOLDER> values with actual credentials
#   - Run: chmod 0600 ~/.vsphere/credentials
#   - The installer will automatically read this file if install-config.yaml doesn't contain credentials
#
YAML_TEMPLATE

# Add vCenter sections
for vcenter in "${VCENTERS[@]}"; do
    cat >> "$CREDENTIALS_FILE" << VCENTER_SECTION

[${vcenter}]
# Main installer account (used during installation only)
user = installer@vsphere.local
password = <INSTALLER_PASSWORD>

# Machine API account (VM lifecycle management)
machine-api.user = ocp-machine-api@vsphere.local
machine-api.password = <MACHINE_API_PASSWORD>

# CSI Driver account (storage provisioning)
csi-driver.user = ocp-csi-driver@vsphere.local
csi-driver.password = <CSI_DRIVER_PASSWORD>

# Cloud Controller Manager account (read-only node discovery)
cloud-controller.user = ocp-cloud-controller@vsphere.local
cloud-controller.password = <CLOUD_CONTROLLER_PASSWORD>

# Diagnostics account (read-only troubleshooting)
diagnostics.user = ocp-diagnostics@vsphere.local
diagnostics.password = <DIAGNOSTICS_PASSWORD>
VCENTER_SECTION
done

# Set secure file permissions
chmod 0600 "$CREDENTIALS_FILE"

log_info "✓ Credentials file generated successfully"
log_info ""
log_info "File location: $CREDENTIALS_FILE"
log_info "File permissions: $(stat -c %a "$CREDENTIALS_FILE" 2>/dev/null || stat -f %A "$CREDENTIALS_FILE")"
log_info ""
log_warn "IMPORTANT: Replace all <PLACEHOLDER> values with actual credentials"
log_info ""
log_info "Next steps:"
log_info "1. Edit $CREDENTIALS_FILE and replace all <PLACEHOLDER> values"
log_info "2. Verify file permissions: ls -la $CREDENTIALS_FILE (should be -rw-------)"
log_info "3. Create install-config.yaml (credentials will be read from this file)"
log_info "4. Run: openshift-install create cluster --dir <install-dir>"
log_info ""
log_info "For multi-vCenter deployments:"
log_info "- Add additional [vcenter-fqdn] sections as needed"
log_info "- Each section should contain credentials for all components on that vCenter"
log_info ""
log_info "Security reminder:"
log_info "- NEVER commit this file to version control"
log_info "- After installation, consider: chmod 0400 $CREDENTIALS_FILE"
log_info "- Or move to a secure vault and delete from filesystem"
log_info ""
log_info "For more information, see: docs/user/vsphere/per-component-credentials.md"
