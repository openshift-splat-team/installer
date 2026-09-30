// Package external extracts External platform metadata from install
// configurations.
package external

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/external"
)

// Metadata converts an install configuration to External metadata.
func Metadata(config *types.InstallConfig) *external.Metadata {
	return &external.Metadata{
		PlatformName: config.Platform.External.PlatformName,
	}
}
