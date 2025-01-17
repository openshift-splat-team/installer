package proxmox

import (
	"github.com/openshift/installer/pkg/types"
	typesproxmox "github.com/openshift/installer/pkg/types/proxmox"
)

// Metadata converts an install configuration to Proxmox metadata.
func Metadata(config *types.InstallConfig) *typesproxmox.Metadata {
	terraformPlatform := "proxmox"

	metadata := &typesproxmox.Metadata{
		TerraformPlatform: terraformPlatform,
	}

	var proxmoxList []typesproxmox.Proxmox
	for _, p := range config.Proxmox.Proxmoxs {
		proxmoxDef := typesproxmox.Proxmox{
			Url:    p.Url,
			Port:   p.Port,
			Token:  p.Token,
			Secret: p.Secret,
		}

		proxmoxList = append(proxmoxList, proxmoxDef)
	}
	metadata.Proxmoxs = proxmoxList

	return metadata
}
