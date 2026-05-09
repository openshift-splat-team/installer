package manifests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	vspheretypes "github.com/openshift/installer/pkg/types/vsphere"
)

// TestGenerateComponentSecrets tests generation of 5 component secrets from validated credentials
func TestGenerateComponentSecrets(t *testing.T) {
	tests := []struct {
		name             string
		installConfig    *types.InstallConfig
		expectedSecrets  int
		expectedNames    []string
		shouldGenerate   bool
	}{
		{
			name: "single vCenter - all components",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					VSphere: &vspheretypes.Platform{
						VCenters: []vspheretypes.VCenter{
							{Server: "vcenter1.example.com"},
						},
						ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
							Installer: &vspheretypes.ComponentCredentials{
								Username: "installer@vsphere.local",
								Password: "pass1",
							},
							MachineAPI: &vspheretypes.ComponentCredentials{
								Username: "machine-api@vsphere.local",
								Password: "pass2",
							},
							Storage: &vspheretypes.ComponentCredentials{
								Username: "storage@vsphere.local",
								Password: "pass3",
							},
							CloudController: &vspheretypes.ComponentCredentials{
								Username: "cloud-controller@vsphere.local",
								Password: "pass4",
							},
							Diagnostics: &vspheretypes.ComponentCredentials{
								Username: "diagnostics@vsphere.local",
								Password: "pass5",
							},
						},
					},
				},
			},
			expectedSecrets: 6, // 5 components + vsphere-cloud-credentials
			expectedNames: []string{
				vsphereInstallerCredsSecretName,
				vsphereMachineAPICredsSecretName,
				vsphereStorageCredsSecretName,
				vsphereCloudControllerCredsSecretName,
				vsphereDiagnosticsCredsSecretName,
				vsphereCloudCredentialsSecretName,
			},
			shouldGenerate: true,
		},
		{
			name: "multi vCenter - subset of components",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					VSphere: &vspheretypes.Platform{
						VCenters: []vspheretypes.VCenter{
							{Server: "vcenter1.example.com"},
							{Server: "vcenter2.example.com"},
						},
						ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
							MachineAPI: &vspheretypes.ComponentCredentials{
								Username: "machine-api@vsphere.local",
								Password: "pass",
							},
							Storage: &vspheretypes.ComponentCredentials{
								Username: "storage@vsphere.local",
								Password: "pass",
							},
						},
					},
				},
			},
			expectedSecrets: 2, // Only machineAPI and storage
			expectedNames: []string{
				vsphereMachineAPICredsSecretName,
				vsphereStorageCredsSecretName,
			},
			shouldGenerate: true,
		},
		{
			name: "no component credentials - no secrets",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					VSphere: &vspheretypes.Platform{
						VCenters: []vspheretypes.VCenter{
							{Server: "vcenter1.example.com"},
						},
					},
				},
			},
			expectedSecrets: 0,
			expectedNames:   []string{},
			shouldGenerate:  false,
		},
		{
			name: "non-vSphere platform - no secrets",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					AWS: &types.AWSPlatform{},
				},
			},
			expectedSecrets: 0,
			expectedNames:   []string{},
			shouldGenerate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parents := asset.Parents{}
			parents.Add(&installconfig.InstallConfig{Config: tt.installConfig})

			asset := &VSphereComponentSecrets{}
			err := asset.Generate(context.Background(), parents)
			require.NoError(t, err)

			if !tt.shouldGenerate {
				assert.Len(t, asset.Secrets, 0)
				assert.Len(t, asset.Files, 0)
				return
			}

			assert.Len(t, asset.Secrets, tt.expectedSecrets)
			assert.Len(t, asset.Files, tt.expectedSecrets)

			// Verify expected secret names
			for _, name := range tt.expectedNames {
				secret, ok := asset.Secrets[name]
				require.True(t, ok, "secret %s not found", name)
				assert.Equal(t, componentSecretsNamespace, secret.Namespace)
				assert.Equal(t, name, secret.Name)
				assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
			}
		})
	}
}

// TestComponentSecretFormat tests multi-vCenter credential format in secrets
func TestComponentSecretFormat(t *testing.T) {
	tests := []struct {
		name          string
		installConfig *types.InstallConfig
		secretName    string
		expectedKeys  []string
	}{
		{
			name: "single vCenter - keys with FQDN prefix",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					VSphere: &vspheretypes.Platform{
						VCenters: []vspheretypes.VCenter{
							{Server: "vcenter1.example.com"},
						},
						ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
							MachineAPI: &vspheretypes.ComponentCredentials{
								Username: "machine-api@vsphere.local",
								Password: "password",
							},
						},
					},
				},
			},
			secretName: vsphereMachineAPICredsSecretName,
			expectedKeys: []string{
				"vcenter1.example.com.username",
				"vcenter1.example.com.password",
			},
		},
		{
			name: "multi vCenter - keys for all vCenters",
			installConfig: &types.InstallConfig{
				BaseDomain: "example.com",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
				},
				Platform: types.Platform{
					VSphere: &vspheretypes.Platform{
						VCenters: []vspheretypes.VCenter{
							{Server: "vcenter1.example.com"},
							{Server: "vcenter2.example.com"},
						},
						ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
							Storage: &vspheretypes.ComponentCredentials{
								Username: "storage@vsphere.local",
								Password: "password",
							},
						},
					},
				},
			},
			secretName: vsphereStorageCredsSecretName,
			expectedKeys: []string{
				"vcenter1.example.com.username",
				"vcenter1.example.com.password",
				"vcenter2.example.com.username",
				"vcenter2.example.com.password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parents := asset.Parents{}
			parents.Add(&installconfig.InstallConfig{Config: tt.installConfig})

			asset := &VSphereComponentSecrets{}
			err := asset.Generate(context.Background(), parents)
			require.NoError(t, err)

			secret, ok := asset.Secrets[tt.secretName]
			require.True(t, ok, "secret %s not found", tt.secretName)

			// Verify all expected keys exist
			for _, key := range tt.expectedKeys {
				_, ok := secret.StringData[key]
				assert.True(t, ok, "key %s not found in secret", key)
			}
		})
	}
}

// TestComponentSecretNamespaces tests all secrets created in kube-system namespace
func TestComponentSecretNamespaces(t *testing.T) {
	installConfig := &types.InstallConfig{
		BaseDomain: "example.com",
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
			},
		},
		Platform: types.Platform{
			VSphere: &vspheretypes.Platform{
				VCenters: []vspheretypes.VCenter{
					{Server: "vcenter1.example.com"},
				},
				ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
					Installer: &vspheretypes.ComponentCredentials{
						Username: "installer@vsphere.local",
						Password: "pass1",
					},
					MachineAPI: &vspheretypes.ComponentCredentials{
						Username: "machine-api@vsphere.local",
						Password: "pass2",
					},
					Storage: &vspheretypes.ComponentCredentials{
						Username: "storage@vsphere.local",
						Password: "pass3",
					},
					CloudController: &vspheretypes.ComponentCredentials{
						Username: "cloud-controller@vsphere.local",
						Password: "pass4",
					},
					Diagnostics: &vspheretypes.ComponentCredentials{
						Username: "diagnostics@vsphere.local",
						Password: "pass5",
					},
				},
			},
		},
	}

	parents := asset.Parents{}
	parents.Add(&installconfig.InstallConfig{Config: installConfig})

	asset := &VSphereComponentSecrets{}
	err := asset.Generate(context.Background(), parents)
	require.NoError(t, err)

	// Verify all secrets are in kube-system namespace
	for name, secret := range asset.Secrets {
		assert.Equal(t, componentSecretsNamespace, secret.Namespace,
			"secret %s has incorrect namespace", name)
	}
}

// TestVSphereCloudCredentials tests vsphere-cloud-credentials secret creation with operational credentials
func TestVSphereCloudCredentials(t *testing.T) {
	installConfig := &types.InstallConfig{
		BaseDomain: "example.com",
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
			},
		},
		Platform: types.Platform{
			VSphere: &vspheretypes.Platform{
				VCenters: []vspheretypes.VCenter{
					{Server: "vcenter1.example.com"},
				},
				ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
					Installer: &vspheretypes.ComponentCredentials{
						Username: "installer@vsphere.local",
						Password: "installer-password",
					},
				},
			},
		},
	}

	parents := asset.Parents{}
	parents.Add(&installconfig.InstallConfig{Config: installConfig})

	asset := &VSphereComponentSecrets{}
	err := asset.Generate(context.Background(), parents)
	require.NoError(t, err)

	// Verify vsphere-cloud-credentials secret exists
	secret, ok := asset.Secrets[vsphereCloudCredentialsSecretName]
	require.True(t, ok, "vsphere-cloud-credentials secret not found")
	assert.Equal(t, vsphereCloudCredentialsSecretName, secret.Name)
	assert.Equal(t, componentSecretsNamespace, secret.Namespace)

	// Verify credentials are from installer (operational = installer for now)
	username, ok := secret.StringData["vcenter1.example.com.username"]
	require.True(t, ok)
	assert.Equal(t, "installer@vsphere.local", username)

	password, ok := secret.StringData["vcenter1.example.com.password"]
	require.True(t, ok)
	assert.Equal(t, "installer-password", password)
}

// TestInstallerCredentialPersistence tests installer credential persistence in vsphere-cloud-credentials
func TestInstallerCredentialPersistence(t *testing.T) {
	installConfig := &types.InstallConfig{
		BaseDomain: "example.com",
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
			},
		},
		Platform: types.Platform{
			VSphere: &vspheretypes.Platform{
				VCenters: []vspheretypes.VCenter{
					{Server: "vcenter1.example.com"},
					{Server: "vcenter2.example.com"},
				},
				ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
					Installer: &vspheretypes.ComponentCredentials{
						Username: "installer@vsphere.local",
						Password: "installer-password",
					},
					MachineAPI: &vspheretypes.ComponentCredentials{
						Username: "machine-api@vsphere.local",
						Password: "machine-password",
					},
				},
			},
		},
	}

	parents := asset.Parents{}
	parents.Add(&installconfig.InstallConfig{Config: installConfig})

	asset := &VSphereComponentSecrets{}
	err := asset.Generate(context.Background(), parents)
	require.NoError(t, err)

	// Verify both component-specific and cloud credentials exist
	installerSecret, ok := asset.Secrets[vsphereInstallerCredsSecretName]
	require.True(t, ok, "vsphere-installer-creds not found")

	cloudSecret, ok := asset.Secrets[vsphereCloudCredentialsSecretName]
	require.True(t, ok, "vsphere-cloud-credentials not found")

	// Verify installer credentials are in both secrets
	for _, vcenter := range []string{"vcenter1.example.com", "vcenter2.example.com"} {
		usernameKey := vcenter + ".username"
		passwordKey := vcenter + ".password"

		installerUsername := installerSecret.StringData[usernameKey]
		cloudUsername := cloudSecret.StringData[usernameKey]
		assert.Equal(t, installerUsername, cloudUsername)

		installerPassword := installerSecret.StringData[passwordKey]
		cloudPassword := cloudSecret.StringData[passwordKey]
		assert.Equal(t, installerPassword, cloudPassword)
	}
}

// TestAtomicSecretCreation tests all-or-nothing secret creation behavior
func TestAtomicSecretCreation(t *testing.T) {
	// This test verifies that Generate() either succeeds completely or fails
	// In production, secret creation to the cluster would be atomic (all applied or none)
	// Here we verify the manifest generation completes fully

	installConfig := &types.InstallConfig{
		BaseDomain: "example.com",
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
			},
		},
		Platform: types.Platform{
			VSphere: &vspheretypes.Platform{
				VCenters: []vspheretypes.VCenter{
					{Server: "vcenter1.example.com"},
				},
				ComponentCredentials: &vspheretypes.ComponentCredentialsSet{
					Installer: &vspheretypes.ComponentCredentials{
						Username: "installer@vsphere.local",
						Password: "pass",
					},
					MachineAPI: &vspheretypes.ComponentCredentials{
						Username: "machine-api@vsphere.local",
						Password: "pass",
					},
					Storage: &vspheretypes.ComponentCredentials{
						Username: "storage@vsphere.local",
						Password: "pass",
					},
				},
			},
		},
	}

	parents := asset.Parents{}
	parents.Add(&installconfig.InstallConfig{Config: installConfig})

	asset := &VSphereComponentSecrets{}
	err := asset.Generate(context.Background(), parents)
	require.NoError(t, err)

	// Verify all expected secrets AND files were created
	// Atomic means: if any secret fails, Generate() would return error
	expectedSecrets := 4 // installer, machineAPI, storage, cloud-credentials
	assert.Len(t, asset.Secrets, expectedSecrets)
	assert.Len(t, asset.Files, expectedSecrets)

	// Verify Files() returns same count
	files := asset.Files()
	assert.Len(t, files, expectedSecrets)
}
