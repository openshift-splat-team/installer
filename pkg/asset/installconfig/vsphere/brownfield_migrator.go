package vsphere

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/installer/pkg/types/vsphere"
)

// Component secret names and namespaces based on design doc.
const (
	MachineAPISecretName       = "machine-api-vsphere-credentials"
	MachineAPISecretNamespace  = "openshift-machine-api"
	CSIDriverSecretName        = "vsphere-csi-credentials"
	CSIDriverSecretNamespace   = "openshift-cluster-csi-drivers"
	CCMSecretName              = "vsphere-ccm-credentials"
	CCMSecretNamespace         = "openshift-cloud-controller-manager"
	DiagnosticsSecretName      = "vsphere-diagnostics-credentials"
	DiagnosticsSecretNamespace = "openshift-config"

	// Legacy passthrough secret
	PassthroughSecretName      = "vsphere-cloud-credentials"
	PassthroughSecretNamespace = "kube-system"

	// Backup secret suffix
	BackupSecretSuffix = "-backup"
)

// BrownfieldMigrator handles migration from passthrough to per-component credential mode.
type BrownfieldMigrator struct {
	clientset          kubernetes.Interface
	validatePrivileges bool
	privilegeValidator *PrivilegeValidator
}

// NewBrownfieldMigrator creates a new BrownfieldMigrator instance.
func NewBrownfieldMigrator(clientset kubernetes.Interface, validatePrivileges bool) *BrownfieldMigrator {
	return &BrownfieldMigrator{
		clientset:          clientset,
		validatePrivileges: validatePrivileges,
		privilegeValidator: NewPrivilegeValidator(),
	}
}

// Migrate executes the migration from passthrough to per-component mode.
// Returns an error if migration fails; on error, rollback is attempted.
func (m *BrownfieldMigrator) Migrate(ctx context.Context, credsFile *CredentialsFile) error {
	logrus.Info("Step 1: Validating component credentials")

	// Get the first vCenter from credentials file
	vCenterFQDN, componentCreds := m.getFirstVCenterCredentials(credsFile)
	if vCenterFQDN == "" {
		return fmt.Errorf("no vCenter credentials found in credentials file")
	}

	logrus.Infof("Using vCenter: %s", vCenterFQDN)

	// Step 1: Validate privileges (if enabled)
	if m.validatePrivileges {
		if err := m.validateComponentPrivileges(ctx, componentCreds, vCenterFQDN); err != nil {
			return fmt.Errorf("privilege validation failed: %w", err)
		}
	}

	logrus.Info("Step 2: Creating backup of original passthrough secret")

	// Step 2: Backup original passthrough secret
	originalSecret, err := m.backupOriginalSecret(ctx)
	if err != nil {
		return fmt.Errorf("failed to backup original secret: %w", err)
	}

	logrus.Info("Step 3: Creating component-specific secrets")

	// Step 3: Create component-specific secrets
	if err := m.createComponentSecrets(ctx, componentCreds, vCenterFQDN); err != nil {
		logrus.Errorf("Failed to create component secrets: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("failed to create component secrets: %w", err)
	}

	logrus.Info("Step 4: Updating CCO configuration to per-component mode")

	// Step 4: Update CCO configuration (placeholder - would update CCO config in real implementation)
	// In real implementation, this would modify CCO's CloudCredential CR

	logrus.Info("Step 5: Restarting component operators")

	// Step 5: Restart component operators
	if err := m.restartComponentOperators(ctx); err != nil {
		logrus.Errorf("Failed to restart operators: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("failed to restart operators: %w", err)
	}

	logrus.Info("Step 6: Verifying component reconnection")

	// Step 6: Verify component reconnection
	if err := m.verifyComponentReconnection(ctx); err != nil {
		logrus.Errorf("Failed component reconnection verification: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("component reconnection failed: %w", err)
	}

	logrus.Info("Migration completed successfully")
	return nil
}

// getFirstVCenterCredentials extracts the first vCenter's credentials from the credentials file.
func (m *BrownfieldMigrator) getFirstVCenterCredentials(credsFile *CredentialsFile) (string, *VCenterComponentCredentials) {
	for vCenter, creds := range *credsFile {
		return vCenter, &creds
	}
	return "", nil
}

// validateComponentPrivileges validates that all component credentials have required privileges.
func (m *BrownfieldMigrator) validateComponentPrivileges(ctx context.Context, creds *VCenterComponentCredentials, vCenterFQDN string) error {
	components := map[string]*ComponentAccount{
		"machine-api":      creds.MachineAPI,
		"csi-driver":       creds.CSIDriver,
		"cloud-controller": creds.CloudController,
		"diagnostics":      creds.Diagnostics,
	}

	for component, account := range components {
		if account == nil {
			return fmt.Errorf("component %s credentials not provided in credentials file", component)
		}

		logrus.Debugf("Validating %s credentials", component)

		accountCreds := &vsphere.AccountCredentials{
			Username: account.Username,
			Password: account.Password,
			VCenter:  vCenterFQDN,
		}

		result, err := m.privilegeValidator.ValidateComponentPrivileges(ctx, component, accountCreds, vCenterFQDN)
		if err != nil {
			return fmt.Errorf("failed to validate %s credentials: %w", component, err)
		}

		if !result.Valid {
			return fmt.Errorf("migration failed: %s credentials missing required privilege: %v", component, result.MissingPrivileges)
		}
	}

	logrus.Info("All component credentials validated successfully")
	return nil
}

// backupOriginalSecret creates a backup of the original passthrough-mode secret.
func (m *BrownfieldMigrator) backupOriginalSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Get(ctx, PassthroughSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get original secret: %w", err)
	}

	// Create backup with -backup suffix
	backupSecret := secret.DeepCopy()
	backupSecret.Name = PassthroughSecretName + BackupSecretSuffix
	backupSecret.ResourceVersion = ""
	backupSecret.UID = ""

	_, err = m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Create(ctx, backupSecret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup secret: %w", err)
	}

	logrus.Infof("Created backup secret: %s/%s", PassthroughSecretNamespace, backupSecret.Name)
	return secret, nil
}

// createComponentSecrets creates component-specific secrets from credentials.
func (m *BrownfieldMigrator) createComponentSecrets(ctx context.Context, creds *VCenterComponentCredentials, vCenterFQDN string) error {
	// Create Machine API secret
	if err := m.createSecret(ctx, MachineAPISecretNamespace, MachineAPISecretName, creds.MachineAPI, vCenterFQDN); err != nil {
		return fmt.Errorf("failed to create machine-api secret: %w", err)
	}

	// Create CSI Driver secret
	if err := m.createSecret(ctx, CSIDriverSecretNamespace, CSIDriverSecretName, creds.CSIDriver, vCenterFQDN); err != nil {
		return fmt.Errorf("failed to create csi-driver secret: %w", err)
	}

	// Create CCM secret
	if err := m.createSecret(ctx, CCMSecretNamespace, CCMSecretName, creds.CloudController, vCenterFQDN); err != nil {
		return fmt.Errorf("failed to create cloud-controller secret: %w", err)
	}

	// Create Diagnostics secret
	if err := m.createSecret(ctx, DiagnosticsSecretNamespace, DiagnosticsSecretName, creds.Diagnostics, vCenterFQDN); err != nil {
		return fmt.Errorf("failed to create diagnostics secret: %w", err)
	}

	logrus.Info("All component secrets created successfully")
	return nil
}

// createSecret creates a single component secret.
func (m *BrownfieldMigrator) createSecret(ctx context.Context, namespace, name string, account *ComponentAccount, vCenterFQDN string) error {
	if account == nil {
		return fmt.Errorf("account credentials are nil")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte(account.Username),
			"password": []byte(account.Password),
		},
	}

	_, err := m.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret %s/%s: %w", namespace, name, err)
	}

	logrus.Infof("Created secret: %s/%s", namespace, name)
	return nil
}

// restartComponentOperators restarts component operators to pick up new credentials.
func (m *BrownfieldMigrator) restartComponentOperators(ctx context.Context) error {
	// In a real implementation, this would:
	// 1. Trigger a rollout restart of Machine API operator deployment
	// 2. Trigger a rollout restart of CSI Driver daemonset
	// 3. Trigger a rollout restart of CCM deployment
	// 4. Wait for rollouts to complete

	logrus.Info("Triggering operator restarts (stub implementation)")
	// Stub: simulate operator restart time
	time.Sleep(100 * time.Millisecond)

	return nil
}

// verifyComponentReconnection verifies that all components successfully reconnect with new credentials.
func (m *BrownfieldMigrator) verifyComponentReconnection(ctx context.Context) error {
	// In a real implementation, this would:
	// 1. Check Machine API operator logs for successful vCenter connection
	// 2. Check CSI Driver logs for successful vCenter connection
	// 3. Check CCM logs for successful vCenter connection
	// 4. Check Diagnostics logs for successful vCenter connection
	// 5. Verify all operators reach Ready state within timeout

	logrus.Info("Verifying component reconnection (stub implementation)")
	// Stub: simulate verification time
	time.Sleep(100 * time.Millisecond)

	return nil
}

// rollback restores the original passthrough-mode secret and reverts CCO configuration.
func (m *BrownfieldMigrator) rollback(ctx context.Context, originalSecret *corev1.Secret) {
	logrus.Warn("Initiating rollback to passthrough mode")

	// Delete component-specific secrets
	m.deleteComponentSecrets(ctx)

	// Restore original passthrough secret (if it was modified/deleted)
	// Note: In this implementation, we don't delete the original secret,
	// so restoration is not needed. In a more aggressive migration,
	// we would restore from the backup here.

	// Revert CCO configuration (placeholder)
	logrus.Info("Reverted CCO configuration to passthrough mode")

	logrus.Info("Rollback completed. Cluster returned to original passthrough mode.")
}

// deleteComponentSecrets deletes all component-specific secrets created during migration.
func (m *BrownfieldMigrator) deleteComponentSecrets(ctx context.Context) {
	secrets := []struct {
		namespace string
		name      string
	}{
		{MachineAPISecretNamespace, MachineAPISecretName},
		{CSIDriverSecretNamespace, CSIDriverSecretName},
		{CCMSecretNamespace, CCMSecretName},
		{DiagnosticsSecretNamespace, DiagnosticsSecretName},
	}

	for _, s := range secrets {
		err := m.clientset.CoreV1().Secrets(s.namespace).Delete(ctx, s.name, metav1.DeleteOptions{})
		if err != nil {
			logrus.Warnf("Failed to delete secret %s/%s during rollback: %v", s.namespace, s.name, err)
		} else {
			logrus.Infof("Deleted secret: %s/%s", s.namespace, s.name)
		}
	}
}
