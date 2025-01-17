package proxmox

import (
	"github.com/luthermonson/go-proxmox"
	"sync"
)

type Metadata struct {
	ProxmoxCredentials *proxmox.Credentials

	/*
		sessions    map[string]*session.Session
		credentials map[string]*session.Params

		VCenterContexts map[string]VCenterContext

		VCenterCredentials map[string]VCenterCredential

	*/

	mutex sync.Mutex
}

func NewMetadata() *Metadata {

	return &Metadata{}
}
