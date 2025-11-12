// Package proxmox collects Proxmox-specific configuration.
package proxmox

import (
	"github.com/openshift/installer/pkg/types/proxmox"
)

// Platform collects Proxmox-specific configuration.
// For Proxmox, configuration is typically provided via install-config.yaml
// rather than interactively. This function returns an empty platform to
// maintain consistency with other platform implementations.
func Platform() (*proxmox.Platform, error) {
	return &proxmox.Platform{}, nil
}
