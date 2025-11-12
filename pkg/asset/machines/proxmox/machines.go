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

const (
	masterRole = "master"
)

// GenerateMachines returns a list of CAPI machines for Proxmox.
func GenerateMachines(ctx context.Context, clusterID string, config *types.InstallConfig, pool *types.MachinePool, role string) ([]*asset.RuntimeFile, error) {
	if config.Platform.Proxmox == nil {
		return nil, fmt.Errorf("proxmox platform configuration is required")
	}

	result := make([]*asset.RuntimeFile, 0, pool.Replicas)

	// Get machine pool platform configuration
	poolPlatform := pool.Platform.Proxmox
	if poolPlatform == nil {
		poolPlatform = &proxmoxtypes.MachinePool{}
	}

	// Apply defaults from platform default machine pool
	if config.Platform.Proxmox.DefaultMachinePlatform != nil {
		poolPlatform.Set(config.Platform.Proxmox.DefaultMachinePlatform)
	}

	for i := int64(0); i < *pool.Replicas; i++ {
		machineName := fmt.Sprintf("%s-%s-%d", clusterID, pool.Name, i)

		// Create CAPI Machine
		machine := &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: capiutils.Namespace,
				Name:      machineName,
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name": clusterID,
				},
			},
			Spec: capi.MachineSpec{
				ClusterName: clusterID,
				Bootstrap: capi.Bootstrap{
					DataSecretName: ptr.To(fmt.Sprintf("%s-bootstrap", machineName)),
				},
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
					Kind:       "ProxmoxMachine",
					Name:       machineName,
				},
			},
		}

		if role == masterRole {
			machine.Labels["cluster.x-k8s.io/control-plane"] = ""
		}

		machine.SetGroupVersionKind(capi.GroupVersion.WithKind("Machine"))

		result = append(result, &asset.RuntimeFile{
			File:   asset.File{Filename: fmt.Sprintf("10_inframachine_%s.yaml", machineName)},
			Object: machine,
		})

		// Note: ProxmoxMachine creation is deferred to the manifest generation phase
		// where we have access to the cluster-api-provider-proxmox types.
		// For now, we only generate the CAPI Machine resources.
	}

	return result, nil
}
