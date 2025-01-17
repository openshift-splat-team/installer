package proxmox

import (
	"github.com/openshift/installer/pkg/types/proxmox"
)

// TemplateData holds data specific to templates used for the vsphere platform.
type TemplateData struct {
	// UserProvidedIPs specifies whether users provided IP addresses in the install config.
	UserProvidedVIPs bool
}

// GetTemplateData returns platform-specific data for bootstrap templates.
func GetTemplateData(config *proxmox.Platform) *TemplateData {
	var templateData TemplateData

	templateData.UserProvidedVIPs = len(config.APIVIPs) > 0

	return &templateData
}
