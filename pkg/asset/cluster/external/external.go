// Package external extracts External platform metadata from install
// configurations.
package external

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/external"
)

// Metadata converts an install configuration to External metadata.
//
// The Cluster API provider block is recorded only when the platform asked for
// one. Without it `destroy cluster` has no way to start the provider that
// created the infrastructure, and the installer has no SDK of its own to fall
// back on.
func Metadata(config *types.InstallConfig) *external.Metadata {
	metadata := &external.Metadata{
		PlatformName: config.Platform.External.PlatformName,
	}

	if capi := config.Platform.External.ClusterAPI; capi != nil {
		metadata.ClusterAPI = &external.ClusterAPIMetadata{
			Name:           capi.Name,
			BinaryPath:     capi.BinaryPath,
			ComponentsPath: capi.ComponentsPath,
			Args:           append([]string(nil), capi.Args...),
		}
	}

	return metadata
}
