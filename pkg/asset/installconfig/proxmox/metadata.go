package proxmox

import (
	"github.com/luthermonson/go-proxmox"
	"sync"
)

// todo: proxmox auth stuff here
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
