package vsphere_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/openshift/installer/pkg/types"
	typesvsphere "github.com/openshift/installer/pkg/types/vsphere"
)

// parseVCenter is a test helper that deserializes a minimal install-config YAML
// (via sigs.k8s.io/yaml, which honours json: struct tags) and returns the
// first VCenter entry.
func parseVCenter(t *testing.T, raw string) *typesvsphere.VCenter {
	t.Helper()

	var ic types.InstallConfig
	require.NoError(t, sigsyaml.Unmarshal([]byte(raw), &ic), "yaml.Unmarshal")
	require.NotEmpty(t, ic.Platform.VSphere.VCenters, "expected at least one vCenter")
	return &ic.Platform.VSphere.VCenters[0]
}

// ---------------------------------------------------------------------------
// AC1 — Schema Parsing: componentCredentials block
// ---------------------------------------------------------------------------

func TestComponentCredentialsParsing_FullBlock(t *testing.T) {
	raw := `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter.example.com
        user: installer@vsphere.local
        password: installpass
        datacenters:
          - DC0
        componentCredentials:
          machineAPI:
            user: machine-api@vsphere.local
            password: mapi-pass
          csiDriver:
            user: csi@vsphere.local
            password: csi-pass
          cloudController:
            user: cc@vsphere.local
            password: cc-pass
          diagnostics:
            user: diag@vsphere.local
            password: diag-pass
`
	vc := parseVCenter(t, raw)
	require.NotNil(t, vc.ComponentCredentials)

	assert.Equal(t, "machine-api@vsphere.local", vc.ComponentCredentials.MachineAPI.User)
	assert.Equal(t, "mapi-pass", vc.ComponentCredentials.MachineAPI.Password)
	assert.Equal(t, "csi@vsphere.local", vc.ComponentCredentials.CSIDriver.User)
	assert.Equal(t, "csi-pass", vc.ComponentCredentials.CSIDriver.Password)
	assert.Equal(t, "cc@vsphere.local", vc.ComponentCredentials.CloudController.User)
	assert.Equal(t, "cc-pass", vc.ComponentCredentials.CloudController.Password)
	assert.Equal(t, "diag@vsphere.local", vc.ComponentCredentials.Diagnostics.User)
	assert.Equal(t, "diag-pass", vc.ComponentCredentials.Diagnostics.Password)
}

func TestComponentCredentialsParsing_PartialBlock(t *testing.T) {
	raw := `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter.example.com
        user: installer@vsphere.local
        password: installpass
        datacenters:
          - DC0
        componentCredentials:
          machineAPI:
            user: machine-api@vsphere.local
            password: mapi-pass
`
	vc := parseVCenter(t, raw)
	require.NotNil(t, vc.ComponentCredentials)

	assert.NotNil(t, vc.ComponentCredentials.MachineAPI)
	assert.Equal(t, "machine-api@vsphere.local", vc.ComponentCredentials.MachineAPI.User)
	assert.Nil(t, vc.ComponentCredentials.CSIDriver)
	assert.Nil(t, vc.ComponentCredentials.CloudController)
	assert.Nil(t, vc.ComponentCredentials.Diagnostics)
}

func TestComponentCredentialsParsing_NoBlock(t *testing.T) {
	raw := `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter.example.com
        user: installer@vsphere.local
        password: installpass
        datacenters:
          - DC0
`
	vc := parseVCenter(t, raw)
	assert.Nil(t, vc.ComponentCredentials, "missing componentCredentials block should parse as nil")
}

func TestComponentCredentialsParsing_EmptyBlock(t *testing.T) {
	raw := `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter.example.com
        user: installer@vsphere.local
        password: installpass
        datacenters:
          - DC0
        componentCredentials: {}
`
	vc := parseVCenter(t, raw)
	// Empty block: struct is non-nil but all component fields are nil.
	require.NotNil(t, vc.ComponentCredentials)
	assert.Nil(t, vc.ComponentCredentials.MachineAPI)
	assert.Nil(t, vc.ComponentCredentials.CSIDriver)
	assert.Nil(t, vc.ComponentCredentials.CloudController)
	assert.Nil(t, vc.ComponentCredentials.Diagnostics)
}

func TestComponentCredentialsParsing_AdversarialCases(t *testing.T) {
	cases := []struct {
		name         string
		user         string
		password     string
		wantParseErr bool
	}{
		{
			name:     "special chars in user and password",
			user:     "user+tag@domain.local",
			password: "p@ss!w0rd#$",
		},
		{
			name:     "unicode in password",
			user:     "user@vsphere.local",
			password: "pässwörд",
		},
		{
			// Empty string is valid YAML; validation (not parsing) should reject it.
			name:     "empty username accepted by parser",
			user:     "",
			password: "somepass",
		},
		{
			name:     "empty password accepted by parser",
			user:     "user@vsphere.local",
			password: "",
		},
		{
			name:     "very long username (1025 chars)",
			user:     strings.Repeat("a", 1025),
			password: "pass",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := buildInstallConfigYAML(tc.user, tc.password)
			var ic types.InstallConfig
			err := sigsyaml.Unmarshal([]byte(raw), &ic)
			if tc.wantParseErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "YAML parsing must not error for adversarial inputs — validation happens later")
			}
		})
	}
}

func TestComponentCredentialsParsing_MultipleVCenters(t *testing.T) {
	raw := `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter1.example.com
        user: installer@vsphere.local
        password: pass1
        datacenters:
          - DC0
        componentCredentials:
          machineAPI:
            user: mapi-vc1@vsphere.local
            password: mapi-pass-vc1
      - server: vcenter2.example.com
        user: installer@vsphere.local
        password: pass2
        datacenters:
          - DC1
        componentCredentials:
          machineAPI:
            user: mapi-vc2@vsphere.local
            password: mapi-pass-vc2
`
	var ic types.InstallConfig
	require.NoError(t, sigsyaml.Unmarshal([]byte(raw), &ic))
	require.Len(t, ic.Platform.VSphere.VCenters, 2)

	vc1 := ic.Platform.VSphere.VCenters[0]
	vc2 := ic.Platform.VSphere.VCenters[1]

	require.NotNil(t, vc1.ComponentCredentials)
	require.NotNil(t, vc2.ComponentCredentials)
	assert.Equal(t, "mapi-vc1@vsphere.local", vc1.ComponentCredentials.MachineAPI.User)
	assert.Equal(t, "mapi-vc2@vsphere.local", vc2.ComponentCredentials.MachineAPI.User)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildInstallConfigYAML(user, password string) string {
	return `
metadata:
  name: test-cluster
baseDomain: example.com
platform:
  vsphere:
    vcenters:
      - server: vcenter.example.com
        user: installer@vsphere.local
        password: installpass
        datacenters:
          - DC0
        componentCredentials:
          machineAPI:
            user: ` + user + `
            password: ` + password + `
`
}

// Compile-time reference to keep import alive.
var _ = typesvsphere.ComponentCredentials{}
