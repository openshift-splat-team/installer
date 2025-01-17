package proxmox

import (
	"context"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/proxmox"
)

func GenerateMachines(ctx context.Context, clusterID string, config *types.InstallConfig, pool *types.MachinePool, osImage string, role string, metadata *proxmox.Metadata) ([]*asset.RuntimeFile, error) {

	return nil, nil

}
