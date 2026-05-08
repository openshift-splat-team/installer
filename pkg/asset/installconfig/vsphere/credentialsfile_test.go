package vsphere

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typesvsphere "github.com/openshift/installer/pkg/types/vsphere"
)

// ---------------------------------------------------------------------------
// AC2 — ~/.vsphere/credentials File Permission Validation
// ---------------------------------------------------------------------------

func TestCredentialsFilePermissions(t *testing.T) {
	cases := []struct {
		name        string
		fileMode    os.FileMode
		dirMode     os.FileMode
		wantErr     bool
		errContains string
	}{
		{
			name:     "0600 file 0700 dir accepted",
			fileMode: 0600,
			dirMode:  0700,
			wantErr:  false,
		},
		{
			name:        "0644 file rejected",
			fileMode:    0644,
			dirMode:     0700,
			wantErr:     true,
			errContains: "insecure permissions (0644). Must be 0600",
		},
		{
			name:        "0666 file rejected",
			fileMode:    0666,
			dirMode:     0700,
			wantErr:     true,
			errContains: "insecure permissions",
		},
		{
			name:        "0777 file rejected",
			fileMode:    0777,
			dirMode:     0700,
			wantErr:     true,
			errContains: "insecure permissions",
		},
		{
			// Directory too permissive, even with correct file mode.
			name:        "directory 0755 rejected",
			fileMode:    0600,
			dirMode:     0755,
			wantErr:     true,
			errContains: "insecure permissions",
		},
		{
			// 0400 (owner-read-only) is more restrictive than 0600.
			// The validator requires exactly 0600, so this is rejected.
			name:        "0400 file rejected (not exactly 0600)",
			fileMode:    0400,
			dirMode:     0700,
			wantErr:     true,
			errContains: "insecure permissions (0400). Must be 0600",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create an isolated temp dir so tests cannot interfere.
			base := t.TempDir()
			credDir := filepath.Join(base, ".vsphere")
			require.NoError(t, os.Mkdir(credDir, 0700))
			require.NoError(t, os.Chmod(credDir, tc.dirMode))

			credFile := filepath.Join(credDir, "credentials")
			require.NoError(t, os.WriteFile(credFile, []byte("user: test\npassword: test\n"), tc.fileMode))

			err := ValidateCredentialsFilePermissions(credFile)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCredentialsFile_NotFound(t *testing.T) {
	err := ValidateCredentialsFilePermissions("/nonexistent/path/.vsphere/credentials")
	require.NoError(t, err, "missing credentials file must not be an error")
}

func TestCredentialsFile_Symlink_TargetInsecure(t *testing.T) {
	base := t.TempDir()
	credDir := filepath.Join(base, ".vsphere")
	require.NoError(t, os.Mkdir(credDir, 0700))

	// Create a target file with insecure permissions.
	target := filepath.Join(base, "real-credentials")
	require.NoError(t, os.WriteFile(target, []byte("user: test\n"), 0644))

	// Create a symlink inside the .vsphere dir pointing to the insecure target.
	link := filepath.Join(credDir, "credentials")
	require.NoError(t, os.Symlink(target, link))

	// ValidateCredentialsFilePermissions uses os.Stat which follows the symlink.
	err := ValidateCredentialsFilePermissions(link)
	require.Error(t, err, "symlink to 0644 target must be rejected")
	assert.Contains(t, err.Error(), "insecure permissions")
}

// ---------------------------------------------------------------------------
// AC3 — Credential Precedence: install-config.yaml > ~/.vsphere/credentials
// ---------------------------------------------------------------------------

func credential(user, password string) *typesvsphere.Credential {
	return &typesvsphere.Credential{User: user, Password: password}
}

func TestCredentialPrecedence_InstallConfigWins(t *testing.T) {
	ic := &typesvsphere.ComponentCredentials{
		MachineAPI: credential("ic-mapi@vsphere.local", "ic-mapi-pass"),
	}
	file := &typesvsphere.ComponentCredentials{
		MachineAPI: credential("file-mapi@vsphere.local", "file-mapi-pass"),
	}
	resolved := ResolveComponentCredentials(ic, file)
	require.NotNil(t, resolved.MachineAPI)
	assert.Equal(t, "ic-mapi@vsphere.local", resolved.MachineAPI.User)
}

func TestCredentialPrecedence_PerComponentSplit(t *testing.T) {
	ic := &typesvsphere.ComponentCredentials{
		MachineAPI: credential("ic-mapi@vsphere.local", "ic-mapi-pass"),
	}
	file := &typesvsphere.ComponentCredentials{
		CSIDriver: credential("file-csi@vsphere.local", "file-csi-pass"),
	}
	resolved := ResolveComponentCredentials(ic, file)

	require.NotNil(t, resolved.MachineAPI)
	assert.Equal(t, "ic-mapi@vsphere.local", resolved.MachineAPI.User)

	require.NotNil(t, resolved.CSIDriver)
	assert.Equal(t, "file-csi@vsphere.local", resolved.CSIDriver.User)
}

func TestCredentialPrecedence_FileOnlyWhenNoInstallConfig(t *testing.T) {
	file := &typesvsphere.ComponentCredentials{
		MachineAPI:      credential("file-mapi@vsphere.local", "pass"),
		CSIDriver:       credential("file-csi@vsphere.local", "pass"),
		CloudController: credential("file-cc@vsphere.local", "pass"),
		Diagnostics:     credential("file-diag@vsphere.local", "pass"),
	}
	resolved := ResolveComponentCredentials(nil, file)

	assert.Equal(t, "file-mapi@vsphere.local", resolved.MachineAPI.User)
	assert.Equal(t, "file-csi@vsphere.local", resolved.CSIDriver.User)
	assert.Equal(t, "file-cc@vsphere.local", resolved.CloudController.User)
	assert.Equal(t, "file-diag@vsphere.local", resolved.Diagnostics.User)
}

func TestCredentialPrecedence_InstallConfigOnlyWhenNoFile(t *testing.T) {
	ic := &typesvsphere.ComponentCredentials{
		MachineAPI:      credential("ic-mapi@vsphere.local", "pass"),
		CSIDriver:       credential("ic-csi@vsphere.local", "pass"),
		CloudController: credential("ic-cc@vsphere.local", "pass"),
		Diagnostics:     credential("ic-diag@vsphere.local", "pass"),
	}
	resolved := ResolveComponentCredentials(ic, nil)

	assert.Equal(t, "ic-mapi@vsphere.local", resolved.MachineAPI.User)
	assert.Equal(t, "ic-csi@vsphere.local", resolved.CSIDriver.User)
	assert.Equal(t, "ic-cc@vsphere.local", resolved.CloudController.User)
	assert.Equal(t, "ic-diag@vsphere.local", resolved.Diagnostics.User)
}

func TestCredentialPrecedence_NeitherSource(t *testing.T) {
	resolved := ResolveComponentCredentials(nil, nil)
	assert.NotNil(t, resolved, "result must never be nil")
	assert.Nil(t, resolved.MachineAPI)
	assert.Nil(t, resolved.CSIDriver)
	assert.Nil(t, resolved.CloudController)
	assert.Nil(t, resolved.Diagnostics)
}
