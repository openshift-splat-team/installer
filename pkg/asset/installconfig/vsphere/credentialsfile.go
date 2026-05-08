package vsphere

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// ValidateCredentialsFilePermissions validates that the credentials file at
// path has 0600 permissions and its parent directory has 0700 permissions.
//
// If path does not exist the function returns nil (the file is optional).
// Symlinks are followed via os.Stat so the target's permissions are checked.
func ValidateCredentialsFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to stat credentials file %q: %w", path, err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		return fmt.Errorf("Credentials file has insecure permissions (%04o). Must be 0600.", mode)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("failed to stat credentials directory: %w", err)
	}

	dirMode := dirInfo.Mode().Perm()
	if dirMode != 0700 {
		return fmt.Errorf("credentials directory has insecure permissions (%04o). Must be 0700", dirMode)
	}

	return nil
}

// ResolveComponentCredentials merges install-config.yaml credentials with
// credentials-file credentials using per-component precedence:
// install-config.yaml values win for each component individually.
//
// Either argument may be nil. The returned value is never nil (it may be an
// empty struct when both inputs are nil or empty).
func ResolveComponentCredentials(installConfigCreds, fileCreds *vsphere.ComponentCredentials) *vsphere.ComponentCredentials {
	result := &vsphere.ComponentCredentials{}

	result.MachineAPI = resolveCredential(
		credField(installConfigCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.MachineAPI }),
		credField(fileCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.MachineAPI }),
	)
	result.CSIDriver = resolveCredential(
		credField(installConfigCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.CSIDriver }),
		credField(fileCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.CSIDriver }),
	)
	result.CloudController = resolveCredential(
		credField(installConfigCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.CloudController }),
		credField(fileCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.CloudController }),
	)
	result.Diagnostics = resolveCredential(
		credField(installConfigCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.Diagnostics }),
		credField(fileCreds, func(c *vsphere.ComponentCredentials) *vsphere.Credential { return c.Diagnostics }),
	)

	return result
}

// credField safely extracts a single credential from a ComponentCredentials
// pointer, returning nil when the parent pointer is nil.
func credField(parent *vsphere.ComponentCredentials, fn func(*vsphere.ComponentCredentials) *vsphere.Credential) *vsphere.Credential {
	if parent == nil {
		return nil
	}
	return fn(parent)
}

// resolveCredential picks the install-config credential when non-nil,
// otherwise falls back to the file credential.
func resolveCredential(installConfig, file *vsphere.Credential) *vsphere.Credential {
	if installConfig != nil {
		return installConfig
	}
	return file
}
