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

// Name returns the proxmox provider name.
func (p Provider) Name() string {
	return proxmox.Name
}

// PublicGatherEndpoint indicates that machine ready checks should NOT wait for an ExternalIP
// in the status when declaring machines ready. For Proxmox, we use InternalIP.
func (Provider) PublicGatherEndpoint() clusterapi.GatherEndpoint { return clusterapi.InternalIP }

// PreProvision performs pre-provisioning steps for Proxmox.
// This includes downloading and caching the RHCOS image that will be used
// to create VM templates in Proxmox.
func (p Provider) PreProvision(ctx context.Context, in clusterapi.PreProvisionInput) error {
	// Download and cache the RHCOS image for Proxmox
	// The cached image will be used later to create VM templates
	cachedImage, err := cache.DownloadImageFile(in.RhcosImage.ControlPlane, cache.InstallerApplicationName)
	if err != nil {
		return fmt.Errorf("failed to download and cache RHCOS image: %w", err)
	}

	fmt.Printf("RHCOS image cached at: %s\n", cachedImage)

	// TODO: Additional pre-provisioning steps:
	// 1. Upload the cached image to Proxmox storage
	// 2. Create a VM template from the uploaded image
	// 3. Validate Proxmox API connectivity and permissions
	// 4. Validate that required Proxmox resources (storage, network) exist
	//
	// These steps will require implementing a Proxmox API client similar to
	// what vSphere does with govmomi.

	return nil
}
