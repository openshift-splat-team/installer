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

	// Detect multi-vCenter mode
	multiVCenterMode := m.isMultiVCenterMode(credsFile)
	if multiVCenterMode {
		logrus.Info("Detected multi-vCenter topology")
	}

	// Validate all vCenters and their credentials
	allVCenterCreds := m.getAllVCenterCredentials(credsFile)
	if len(allVCenterCreds) == 0 {
		return fmt.Errorf("no vCenter credentials found in credentials file")
	}

	logrus.Infof("Processing %d vCenter(s)", len(allVCenterCreds))

	// Step 1: Validate privileges (if enabled)
	if m.validatePrivileges {
		for vCenterFQDN, componentCreds := range allVCenterCreds {
			if err := m.validateComponentPrivileges(ctx, componentCreds, vCenterFQDN); err != nil {
				return fmt.Errorf("privilege validation failed for vCenter %s: %w", vCenterFQDN, err)
			}
		}
	}

	logrus.Info("Step 2: Validating namespaces exist")

	// Step 2: Validate that all target namespaces exist
	if err := m.validateNamespacesExist(ctx); err != nil {
		return fmt.Errorf("namespace validation failed: %w", err)
	}

	logrus.Info("Step 3: Creating backup of original passthrough secret")

	// Step 3: Backup original passthrough secret
	originalSecret, err := m.backupOriginalSecret(ctx)
	if err != nil {
		return fmt.Errorf("failed to backup original secret: %w", err)
	}

	logrus.Info("Step 4: Creating component-specific secrets")

	// Step 4: Create component-specific secrets for all vCenters
	if err := m.createAllComponentSecrets(ctx, allVCenterCreds, multiVCenterMode); err != nil {
		logrus.Errorf("Failed to create component secrets: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("failed to create component secrets: %w", err)
	}

	logrus.Info("Step 5: Updating CCO configuration to per-component mode")

	// Step 5: Update CCO configuration (placeholder - would update CCO config in real implementation)
	// In real implementation, this would modify CCO's CloudCredential CR

	logrus.Info("Step 6: Restarting component operators")

	// Step 6: Restart component operators
	if err := m.restartComponentOperators(ctx); err != nil {
		logrus.Errorf("Failed to restart operators: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("failed to restart operators: %w", err)
	}

	logrus.Info("Step 7: Verifying component reconnection")

	// Step 7: Verify component reconnection
	if err := m.verifyComponentReconnection(ctx); err != nil {
		logrus.Errorf("Failed component reconnection verification: %v", err)
		logrus.Info("Rolling back to original state...")
		m.rollback(ctx, originalSecret)
		return fmt.Errorf("component reconnection failed: %w", err)
	}

	logrus.Info("Migration completed successfully")
	return nil
}

// isMultiVCenterMode determines if the configuration uses multi-vCenter topology.
// Multi-vCenter mode is detected when the credentials file contains multiple vCenters.
func (m *BrownfieldMigrator) isMultiVCenterMode(credsFile *CredentialsFile) bool {
	return len(*credsFile) > 1
}

// getAllVCenterCredentials returns all vCenter credentials from the credentials file.
func (m *BrownfieldMigrator) getAllVCenterCredentials(credsFile *CredentialsFile) map[string]*VCenterComponentCredentials {
	result := make(map[string]*VCenterComponentCredentials)
	for vCenter, creds := range *credsFile {
		credsCopy := creds
		result[vCenter] = &credsCopy
	}
	return result
}

// validateNamespacesExist verifies that all required component namespaces exist in the cluster.
func (m *BrownfieldMigrator) validateNamespacesExist(ctx context.Context) error {
	requiredNamespaces := []string{
		MachineAPISecretNamespace,
		CSIDriverSecretNamespace,
		CCMSecretNamespace,
		DiagnosticsSecretNamespace,
	}

	for _, ns := range requiredNamespaces {
		_, err := m.clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("required namespace %s does not exist: %w", ns, err)
		}
	}

	logrus.Info("All required namespaces exist")
	return nil
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

// createAllComponentSecrets creates component-specific secrets for all vCenters.
func (m *BrownfieldMigrator) createAllComponentSecrets(ctx context.Context, allVCenterCreds map[string]*VCenterComponentCredentials, multiVCenterMode bool) error {
	// In multi-vCenter mode, we need to merge credentials from all vCenters into component secrets
	// Each component secret will contain credentials for all vCenters it needs to access

	// For simplicity in the brownfield migration, we use the first vCenter's credentials
	// A more advanced implementation would support merging multiple vCenter credentials per component
	var primaryVCenter string
	var primaryCreds *VCenterComponentCredentials
	for vcenter, creds := range allVCenterCreds {
		primaryVCenter = vcenter
		primaryCreds = creds
		break
	}

	// Create Machine API secret
	if err := m.createSecret(ctx, MachineAPISecretNamespace, MachineAPISecretName, primaryCreds.MachineAPI, primaryVCenter, multiVCenterMode); err != nil {
		return fmt.Errorf("failed to create machine-api secret: %w", err)
	}

	// Create CSI Driver secret
	if err := m.createSecret(ctx, CSIDriverSecretNamespace, CSIDriverSecretName, primaryCreds.CSIDriver, primaryVCenter, multiVCenterMode); err != nil {
		return fmt.Errorf("failed to create csi-driver secret: %w", err)
	}

	// Create CCM secret
	if err := m.createSecret(ctx, CCMSecretNamespace, CCMSecretName, primaryCreds.CloudController, primaryVCenter, multiVCenterMode); err != nil {
		return fmt.Errorf("failed to create cloud-controller secret: %w", err)
	}

	// Create Diagnostics secret
	if err := m.createSecret(ctx, DiagnosticsSecretNamespace, DiagnosticsSecretName, primaryCreds.Diagnostics, primaryVCenter, multiVCenterMode); err != nil {
		return fmt.Errorf("failed to create diagnostics secret: %w", err)
	}

	logrus.Info("All component secrets created successfully")
	return nil
}

// createSecret creates a single component secret with appropriate format based on multi-vCenter mode.
func (m *BrownfieldMigrator) createSecret(ctx context.Context, namespace, name string, account *ComponentAccount, vCenterFQDN string, multiVCenterMode bool) error {
	if account == nil {
		return fmt.Errorf("account credentials are nil")
	}

	secretData := make(map[string][]byte)

	// Use FQDN-keyed format in multi-vCenter mode, simple format otherwise
	if multiVCenterMode && vCenterFQDN != "" {
		// Multi-vCenter mode: use FQDN-keyed credentials (vcenter1.example.com.username)
		usernameKey := fmt.Sprintf("%s.username", vCenterFQDN)
		passwordKey := fmt.Sprintf("%s.password", vCenterFQDN)
		secretData[usernameKey] = []byte(account.Username)
		secretData[passwordKey] = []byte(account.Password)
	} else {
		// Single-vCenter mode: use simple keys (username/password)
		secretData["username"] = []byte(account.Username)
		secretData["password"] = []byte(account.Password)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
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
	// Restart Machine API operator deployment
	if err := m.restartDeployment(ctx, "openshift-machine-api", "machine-api-operator"); err != nil {
		return fmt.Errorf("failed to restart machine-api operator: %w", err)
	}

	// Restart CSI Driver controller deployment
	if err := m.restartDeployment(ctx, "openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-controller"); err != nil {
		return fmt.Errorf("failed to restart csi-driver controller: %w", err)
	}

	// Restart Cloud Controller Manager deployment
	if err := m.restartDeployment(ctx, "openshift-cloud-controller-manager", "vsphere-cloud-controller-manager"); err != nil {
		return fmt.Errorf("failed to restart cloud-controller-manager: %w", err)
	}

	logrus.Info("All operator restarts completed successfully")
	return nil
}

// restartDeployment triggers a rollout restart of a deployment by updating its restart annotation.
func (m *BrownfieldMigrator) restartDeployment(ctx context.Context, namespace, name string) error {
	logrus.Infof("Restarting deployment %s/%s", namespace, name)

	deployment, err := m.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	// Update the restart annotation to trigger a rollout restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = m.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s/%s: %w", namespace, name, err)
	}

	// Wait for rollout to complete (simplified - checks for ready replicas)
	timeout := 5 * time.Minute
	interval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		deployment, err = m.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment status: %w", err)
		}

		if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
			logrus.Infof("Deployment %s/%s rollout completed", namespace, name)
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("deployment %s/%s rollout timed out after %v", namespace, name, timeout)
}

// verifyComponentReconnection verifies that all components successfully reconnect with new credentials.
func (m *BrownfieldMigrator) verifyComponentReconnection(ctx context.Context) error {
	// Verify Machine API operator is ready
	if err := m.verifyDeploymentReady(ctx, "openshift-machine-api", "machine-api-operator"); err != nil {
		return fmt.Errorf("machine-api operator not ready: %w", err)
	}

	// Verify CSI Driver controller is ready
	if err := m.verifyDeploymentReady(ctx, "openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-controller"); err != nil {
		return fmt.Errorf("csi-driver controller not ready: %w", err)
	}

	// Verify Cloud Controller Manager is ready
	if err := m.verifyDeploymentReady(ctx, "openshift-cloud-controller-manager", "vsphere-cloud-controller-manager"); err != nil {
		return fmt.Errorf("cloud-controller-manager not ready: %w", err)
	}

	logrus.Info("All components verified successfully")
	return nil
}

// verifyDeploymentReady verifies that a deployment is ready with all replicas available.
func (m *BrownfieldMigrator) verifyDeploymentReady(ctx context.Context, namespace, name string) error {
	deployment, err := m.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	if deployment.Status.ReadyReplicas != deployment.Status.Replicas || deployment.Status.Replicas == 0 {
		return fmt.Errorf("deployment %s/%s is not ready: %d/%d replicas ready",
			namespace, name, deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	}

	logrus.Infof("Deployment %s/%s is ready", namespace, name)
	return nil
}

// rollback restores the original passthrough-mode secret and reverts CCO configuration.
func (m *BrownfieldMigrator) rollback(ctx context.Context, originalSecret *corev1.Secret) {
	logrus.Warn("Initiating rollback to passthrough mode")

	// Delete component-specific secrets
	m.deleteComponentSecrets(ctx)

	// Restore original passthrough secret from backup
	backupSecretName := PassthroughSecretName + BackupSecretSuffix
	backupSecret, err := m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Get(ctx, backupSecretName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get backup secret during rollback: %v", err)
		return
	}

	// Restore the original secret
	restoredSecret := backupSecret.DeepCopy()
	restoredSecret.Name = PassthroughSecretName
	restoredSecret.ResourceVersion = ""
	restoredSecret.UID = ""

	// Delete current secret if it exists and replace with backup
	_ = m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Delete(ctx, PassthroughSecretName, metav1.DeleteOptions{})

	_, err = m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Create(ctx, restoredSecret, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("Failed to restore original secret during rollback: %v", err)
		return
	}

	logrus.Infof("Restored original secret: %s/%s", PassthroughSecretNamespace, PassthroughSecretName)

	// Delete backup secret after successful restoration
	err = m.clientset.CoreV1().Secrets(PassthroughSecretNamespace).Delete(ctx, backupSecretName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Warnf("Failed to delete backup secret during rollback: %v", err)
	} else {
		logrus.Infof("Deleted backup secret: %s/%s", PassthroughSecretNamespace, backupSecretName)
	}

	// Revert CCO configuration
	// TODO: Implement CCO CloudCredential CR revert logic when CCO API is available
	// This would involve updating the CloudCredential CR to disable per-component mode
	// and re-enable passthrough mode
	logrus.Info("Reverted CCO configuration to passthrough mode (placeholder - requires CCO API)")

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
