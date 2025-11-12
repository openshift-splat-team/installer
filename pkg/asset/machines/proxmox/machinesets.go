package proxmox

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/manifests/capiutils"
	"github.com/openshift/installer/pkg/types"
	proxmoxtypes "github.com/openshift/installer/pkg/types/proxmox"
)

// GenerateMachineSets returns a list of CAPI MachineSets for Proxmox worker nodes.
func GenerateMachineSets(ctx context.Context, clusterID string, config *types.InstallConfig, pool *types.MachinePool) ([]*asset.RuntimeFile, error) {
	if config.Platform.Proxmox == nil {
		return nil, fmt.Errorf("proxmox platform configuration is required")
	}

	// Get machine pool platform configuration
	poolPlatform := pool.Platform.Proxmox
	if poolPlatform == nil {
		poolPlatform = &proxmoxtypes.MachinePool{}
	}

	// Apply defaults from platform default machine pool
	if config.Platform.Proxmox.DefaultMachinePlatform != nil {
		poolPlatform.Set(config.Platform.Proxmox.DefaultMachinePlatform)
	}

	machineSetName := fmt.Sprintf("%s-%s", clusterID, pool.Name)

	machineSet := &capi.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: capiutils.Namespace,
			Name:      machineSetName,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": clusterID,
			},
		},
		Spec: capi.MachineSetSpec{
			ClusterName: clusterID,
			Replicas:    pool.Replicas,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster.x-k8s.io/cluster-name": clusterID,
					"cluster.x-k8s.io/set-name":     machineSetName,
				},
			},
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": clusterID,
						"cluster.x-k8s.io/set-name":     machineSetName,
					},
				},
				Spec: capi.MachineSpec{
					ClusterName: clusterID,
					Bootstrap: capi.Bootstrap{
						DataSecretName: ptr.To(fmt.Sprintf("worker-user-data")),
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
						Kind:       "ProxmoxMachineTemplate",
						Name:       machineSetName,
					},
				},
			},
		},
	}

	machineSet.SetGroupVersionKind(capi.GroupVersion.WithKind("MachineSet"))

	result := []*asset.RuntimeFile{
		{
			File:   asset.File{Filename: fmt.Sprintf("10_machineset_%s.yaml", machineSetName)},
			Object: machineSet,
		},
	}

	// Note: ProxmoxMachineTemplate creation is deferred to the manifest generation phase
	// where we have access to the cluster-api-provider-proxmox types.

	return result, nil
}
