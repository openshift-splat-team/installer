package proxmox

type Platform struct {
	Proxmoxs []Proxmox `json:"proxmoxs"`
}

// todo: do we use baremetal networking (static pods)???

type Proxmox struct {
	Url string `json:"url"`

	Port   int32  `json:"port,omitempty"`
	Token  string `json:"token"`
	Secret string `json:"secret"`
}
