package proxmox

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/manifests/capiutils"
	"github.com/openshift/installer/pkg/types"
)

// GenerateClusterAssets generates the CAPI Cluster manifest for Proxmox.
func GenerateClusterAssets(ctx context.Context, clusterID string, config *types.InstallConfig) (*asset.RuntimeFile, error) {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: capiutils.Namespace,
		},
		Spec: capi.ClusterSpec{
			ClusterNetwork: &capi.ClusterNetwork{
				Pods: &capi.NetworkRanges{
					CIDRBlocks: config.Networking.PodCIDR(),
				},
				Services: &capi.NetworkRanges{
					CIDRBlocks: config.Networking.ServiceCIDR(),
				},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
				Kind:       "ProxmoxCluster",
				Name:       clusterID,
			},
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
				Kind:       "KubeadmControlPlane",
				Name:       fmt.Sprintf("%s-control-plane", clusterID),
			},
		},
	}

	cluster.SetGroupVersionKind(capi.GroupVersion.WithKind("Cluster"))

	return &asset.RuntimeFile{
		File:   asset.File{Filename: "01_cluster.yaml"},
		Object: cluster,
	}, nil
}
