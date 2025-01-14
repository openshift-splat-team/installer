package proxmox

type Platform struct {
	Proxmoxs []string `json:"proxmoxs"`
}

type Proxmox struct {
	Url string `json:"url"`

	Port   int32  `json:"port,omitempty"`
	Token  string `json:"token"`
	Secret string `json:"secret"`
}
