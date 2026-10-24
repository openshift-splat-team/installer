package platform

import (
	"fmt"

	"github.com/openshift/installer/pkg/infrastructure"
	awscapi "github.com/openshift/installer/pkg/infrastructure/aws/clusterapi"
	azureinfra "github.com/openshift/installer/pkg/infrastructure/azure"
	baremetalinfra "github.com/openshift/installer/pkg/infrastructure/baremetal"
	"github.com/openshift/installer/pkg/infrastructure/clusterapi"
	externalcapi "github.com/openshift/installer/pkg/infrastructure/external/clusterapi"
	gcpcapi "github.com/openshift/installer/pkg/infrastructure/gcp/clusterapi"
	ibmcloudcapi "github.com/openshift/installer/pkg/infrastructure/ibmcloud/clusterapi"
	nutanixcapi "github.com/openshift/installer/pkg/infrastructure/nutanix/clusterapi"
	openstackcapi "github.com/openshift/installer/pkg/infrastructure/openstack/clusterapi"
	powervscapi "github.com/openshift/installer/pkg/infrastructure/powervs/clusterapi"
	vspherecapi "github.com/openshift/installer/pkg/infrastructure/vsphere/clusterapi"
	"github.com/openshift/installer/pkg/types"
	awstypes "github.com/openshift/installer/pkg/types/aws"
	azuretypes "github.com/openshift/installer/pkg/types/azure"
	baremetaltypes "github.com/openshift/installer/pkg/types/baremetal"
	externaltypes "github.com/openshift/installer/pkg/types/external"
	"github.com/openshift/installer/pkg/types/featuregates"
	gcptypes "github.com/openshift/installer/pkg/types/gcp"
	ibmcloudtypes "github.com/openshift/installer/pkg/types/ibmcloud"
	nutanixtypes "github.com/openshift/installer/pkg/types/nutanix"
	openstacktypes "github.com/openshift/installer/pkg/types/openstack"
	powervctypes "github.com/openshift/installer/pkg/types/powervc"
	powervstypes "github.com/openshift/installer/pkg/types/powervs"
	vspheretypes "github.com/openshift/installer/pkg/types/vsphere"
)

// ProviderForPlatform returns the stages to run to provision the infrastructure for the specified platform.
//
// installConfig may be nil where it is not available, such as on the
// destroy-bootstrap path, which identifies the platform from cluster metadata.
func ProviderForPlatform(platform string, installConfig *types.InstallConfig, fg featuregates.FeatureGate) (infrastructure.Provider, error) {
	switch platform {
	case awstypes.Name:
		return clusterapi.InitializeProvider(&awscapi.Provider{}), nil
	case azuretypes.Name:
		return clusterapi.InitializeProvider(&azureinfra.Provider{}), nil
	case azuretypes.StackTerraformName:
		return clusterapi.InitializeProvider(&azureinfra.Provider{}), nil
	case baremetaltypes.Name:
		return baremetalinfra.InitializeProvider(), nil
	case gcptypes.Name:
		return clusterapi.InitializeProvider(gcpcapi.Provider{}), nil
	case ibmcloudtypes.Name:
		return clusterapi.InitializeProvider(ibmcloudcapi.Provider{}), nil
	case nutanixtypes.Name:
		return clusterapi.InitializeProvider(nutanixcapi.Provider{}), nil
	case powervstypes.Name:
		return clusterapi.InitializeProvider(&powervscapi.Provider{}), nil
	case openstacktypes.Name, powervctypes.Name:
		return clusterapi.InitializeProvider(openstackcapi.Provider{}), nil
	case vspheretypes.Name:
		return clusterapi.InitializeProvider(vspherecapi.Provider{}), nil
	case externaltypes.Name:
		// Only an External install that asked for a Cluster API provider gets
		// one. Without this guard every pre-existing External install -- which
		// provisions nothing by design -- would be routed into the Cluster API
		// path and fail with a message about missing manifests it never
		// intended to supply, instead of the error it has always produced.
		//
		// installConfig is nil on the destroy-bootstrap path, which reaches
		// this platform only when the metadata records a Cluster API install
		// (pkg/asset/cluster/metadata.go), so there is nothing to check.
		if installConfig != nil && !installConfig.Platform.External.ProvisionsWithClusterAPI() {
			break
		}
		// TODO: gate this once the pilot has run. ProviderForPlatform already
		// takes a featuregates.FeatureGate and does not use it; whether
		// External Cluster API provisioning gates separately from the External
		// platform itself is not yet decided.
		return clusterapi.InitializeProvider(externalcapi.Provider{}), nil
	}
	return nil, fmt.Errorf("platform %q does not support automated infrastructure provisioning", platform)
}
