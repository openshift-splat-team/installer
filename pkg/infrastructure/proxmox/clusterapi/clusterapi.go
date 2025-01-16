package clusterapi

import (
	"context"
	"fmt"

	"github.com/openshift/installer/pkg/infrastructure/clusterapi"
	"github.com/openshift/installer/pkg/rhcos/cache"
	"github.com/openshift/installer/pkg/types/proxmox"
)

type Provider struct {
	clusterapi.InfraProvider
}

var _ clusterapi.PreProvider = Provider{}

// Name returns the vsphere provider name.
func (p Provider) Name() string {
	return proxmox.Name
}

// PublicGatherEndpoint indicates that machine ready checks should NOT wait for an ExternalIP
// in the status when declaring machines ready.
func (Provider) PublicGatherEndpoint() clusterapi.GatherEndpoint { return clusterapi.InternalIP }
func (p Provider) PreProvision(ctx context.Context, in clusterapi.PreProvisionInput) error {

	// todo: import image, is there any other pre-steps?
	// https://github.com/ionos-cloud/cluster-api-provider-proxmox/tree/main

	cachedImage, err := cache.DownloadImageFile(in.RhcosImage.ControlPlane, cache.InstallerApplicationName)
	if err != nil {
		return fmt.Errorf("failed to use cached proxmox image: %w", err)
	}

	fmt.Printf("cached proxmox image: %s\n", cachedImage)

	return nil
}
