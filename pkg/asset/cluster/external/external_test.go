package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/external"
)

func config(platform *external.Platform) *types.InstallConfig {
	return &types.InstallConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "example"},
		BaseDomain: "test.example.com",
		Platform:   types.Platform{External: platform},
	}
}

// TestMetadataWithoutClusterAPI covers the long-standing External install,
// which provisions nothing. It must keep recording nothing but the platform
// name: the ClusterAPI block is what tells `destroy cluster` there is
// infrastructure to hand back to a provider, and inventing one for an install
// that never had a provider would send destroy looking for a binary that was
// never configured.
func TestMetadataWithoutClusterAPI(t *testing.T) {
	got := Metadata(config(&external.Platform{PlatformName: "somecloud"}))

	require.NotNil(t, got)
	assert.Equal(t, "somecloud", got.PlatformName)
	assert.Nil(t, got.ClusterAPI)
}

// TestMetadataRecordsWhatDestroyNeeds is the destroy-coverage test. Every
// field asserted here exists because `destroy cluster` cannot proceed without
// it and cannot recover it from anywhere else: the install-config that
// carried it has been consumed by then.
func TestMetadataRecordsWhatDestroyNeeds(t *testing.T) {
	got := Metadata(config(&external.Platform{
		PlatformName: "somecloud",
		ClusterAPI: &external.ClusterAPIProvider{
			Name:           "reference",
			BinaryPath:     "/opt/provider/controller",
			ComponentsPath: "/opt/provider/components.yaml",
			Args:           []string{"--feature-gates=Something=true"},
			Hooks: &external.Hooks{
				InfraReady: "hooks/infra-hook.sh",
				PreDestroy: "hooks/infra-hook.sh",
			},
		},
	}))

	require.NotNil(t, got.ClusterAPI)
	assert.Equal(t, "reference", got.ClusterAPI.Name)
	assert.Equal(t, "/opt/provider/controller", got.ClusterAPI.BinaryPath)
	assert.Equal(t, "/opt/provider/components.yaml", got.ClusterAPI.ComponentsPath)
	assert.Equal(t, []string{"--feature-gates=Something=true"}, got.ClusterAPI.Args)

	// Without the teardown hook, whatever the provisioning hook created --
	// a DNS zone, by construction outside Cluster API's ownership -- leaks
	// with nothing left in the install directory to identify it.
	require.NotNil(t, got.ClusterAPI.Hooks)
	assert.Equal(t, "hooks/infra-hook.sh", got.ClusterAPI.Hooks.PreDestroy)

	// The teardown hook is handed the same cluster identity the provisioning
	// hook was, and the base domain is the half of it that is not already in
	// ClusterMetadata.
	assert.Equal(t, "test.example.com", got.BaseDomain)
}

// TestMetadataDoesNotAliasTheInstallConfig guards a mutation that would be
// invisible until it corrupted metadata.json: this metadata is marshalled
// long after the function returns, so sharing the install-config's memory
// means a later edit to either shows up in the other.
func TestMetadataDoesNotAliasTheInstallConfig(t *testing.T) {
	hooks := &external.Hooks{InfraReady: "hooks/a.sh", PreDestroy: "hooks/a.sh"}
	args := []string{"--one"}
	platform := &external.Platform{
		ClusterAPI: &external.ClusterAPIProvider{Args: args, Hooks: hooks},
	}

	got := Metadata(config(platform))

	hooks.PreDestroy = "hooks/changed.sh"
	args[0] = "--changed"

	assert.Equal(t, "hooks/a.sh", got.ClusterAPI.Hooks.PreDestroy)
	assert.Equal(t, []string{"--one"}, got.ClusterAPI.Args)
}

// TestMetadataWithoutHooks checks the common case: a provider configured with
// no hooks at all records none, rather than an empty block that would suggest
// destroy has something to run.
func TestMetadataWithoutHooks(t *testing.T) {
	got := Metadata(config(&external.Platform{
		ClusterAPI: &external.ClusterAPIProvider{Name: "reference"},
	}))

	require.NotNil(t, got.ClusterAPI)
	assert.Nil(t, got.ClusterAPI.Hooks)
}
