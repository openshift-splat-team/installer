package proxmox

import (
	"context"
	"fmt"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/installconfig/proxmox"
	"github.com/openshift/installer/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	proxmoxapi "github.com/ionos-cloud/cluster-api-provider-proxmox/api/v1alpha1"
)

func GenerateMachines(ctx context.Context, clusterID string, config *types.InstallConfig, pool *types.MachinePool, osImage string, role string, metadata *proxmox.Metadata) ([]*asset.RuntimeFile, error) {
	machine := &capi.Machine{}

	cappMachines := make([]*proxmoxapi.ProxmoxMachine, 0, 3)

	proxmoxMachine := proxmoxapi.ProxmoxMachine{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: proxmoxapi.ProxmoxMachineSpec{
			VirtualMachineCloneSpec: proxmoxapi.VirtualMachineCloneSpec{
				SourceNode:  "",  // todo: proxmox SourceNode will be interesting to figure out....
				TemplateID:  nil, // integer rep of template
				Description: nil,
				Format:      nil,          // vmdk, qcow2, raw, default is raw
				Full:        ptr.To(true), // full clone
				Pool:        nil,
				SnapName:    nil, // snapshots are evil, please don't
				Storage:     nil, // todo: so confused what to do here...
				Target:      nil, // target host
			},
			ProviderID:       nil, // todo: proxmox _confused_ why would this need to be set?
			VirtualMachineID: nil,
			NumSockets:       pool.Platform.Proxmox.NumSockets,
			NumCores:         pool.Platform.Proxmox.NumCores,
			MemoryMiB:        int32(pool.Platform.Proxmox.MemoryMiB),
			Disks: &proxmoxapi.Storage{
				BootVolume: &proxmoxapi.DiskSize{
					Disk:   "", // todo: proxmox: ugh really, I don't want to figure out the scsi id?!?
					SizeGB: 0,
				},
			},
			Network:          nil,
			VMIDRange:        nil,
			Checks:           nil,
			MetadataSettings: nil,
		},
		Status: proxmoxapi.ProxmoxMachineStatus{},
	}

	fmt.Printf(machine.Name)
	fmt.Printf(proxmoxMachine.Name)

	return nil, nil

}
