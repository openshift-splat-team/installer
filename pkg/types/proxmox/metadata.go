package proxmox

// Metadata contains vSphere metadata (e.g. for uninstalling the cluster).
type Metadata struct {
	// TerraformPlatform is the type...
	TerraformPlatform string `json:"terraform_platform"`
	// VCenters collection of vcenters when multi vcenter support is enabled
	Proxmoxs []Proxmox `json:"proxmoxs"`
}
