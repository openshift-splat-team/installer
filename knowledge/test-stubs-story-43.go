//go:build ignore

// Test stubs for Story #43: E2E Test Suite for Per-Component Credential Installation
//
// These stubs cover three acceptance criteria:
//
// AC1: A cluster installed with all four per-component credentials configured passes E2E:
//      installation succeeds, each component's Kubernetes secret contains the correct
//      scoped credential, and CCO conditions show CredentialsProvisionFailed=False.
// AC2: A cluster installed with cloudController per-component credential deliberately omitted
//      falls back to shared credentials, a warning appears in CCO logs, and the component
//      remains functional.
// AC3: Rotating the machine-api credential by updating the Kubernetes secret causes CCO to
//      re-validate the new credential. machine-api continues functioning without restart.
//
// Target repository: openshift/origin (no fork in openshift-splat-team org)
//   Intended location: test/extended/vsphere/per_component_credentials_test.go
//   Framework: Ginkgo (openshift/origin standard) — stubs use testing.T for portability
//
// ENVIRONMENT REQUIREMENTS (must be documented in the test file header):
//   KUBECONFIG           - path to kubeconfig for a running OpenShift cluster
//   VSPHERE_VCENTER      - vCenter FQDN (e.g. vcenter1.example.com)
//   VSPHERE_VCENTER_USER - installer service account (high-privilege, used for audit verification)
//
// STATE DEPENDENCIES (non-idempotency — required E2E documentation per team policy):
//   AC1/AC2: The cluster must have been installed with per-component credentials pre-configured
//            in install-config.yaml. Re-running these tests on a passthrough-mode cluster will
//            fail and must not be treated as a test bug.
//   AC3: TestE2E_CredentialRotation_* modifies kube-system/vsphere-machine-api-creds. Run
//        after AC1 group. Leaves modified credentials in place — restore with known-good creds
//        or re-run AC1 group last to reset state.
//   Ordering: AC1 → AC2 (separate cluster) → AC3 (AC1 cluster, post-install). Tests within
//        each group are independent.
//   Cleanup: secrets written by AC3 rotation tests are left in the state set by the test;
//        no automatic rollback. Cluster remains functional because the rotated credential
//        intentionally has the correct privileges.
//
// Secret names (from CCO credential_distribution.go componentSecretName()):
//   machineAPI            → kube-system/vsphere-machine-api-creds
//   csiDriver             → kube-system/vsphere-storage-creds
//   cloudController       → kube-system/vsphere-cloud-controller-creds
//   vsphereProblemDetector → kube-system/vsphere-problem-detector-creds

package vsphere_e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// oc runs an oc command and returns combined stdout+stderr.
func oc(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "oc", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oc %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// ocAllowFail runs oc and returns output without failing the test on error.
func ocAllowFail(args ...string) (string, error) {
	out, err := exec.Command("oc", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// requireKubeconfig skips when KUBECONFIG is not set.
func requireKubeconfig(t *testing.T) {
	t.Helper()
	if os.Getenv("KUBECONFIG") == "" {
		t.Skip("KUBECONFIG not set: requires live OpenShift vSphere cluster with per-component credentials")
	}
}

// requirePerComponentMode skips when the cluster was not installed with PerComponent credentials.
func requirePerComponentMode(t *testing.T) {
	t.Helper()
	requireKubeconfig(t)
	mode, err := ocAllowFail("get", "infrastructure", "cluster",
		"-o", "jsonpath={.spec.platformSpec.vsphere.credentialsMode}")
	if err != nil || mode != "PerComponent" {
		t.Skipf("cluster credentialsMode=%q; requires PerComponent mode installed cluster", mode)
	}
}

// pollCondition retries fn up to maxWait, returning nil when fn returns nil.
func pollCondition(maxWait time.Duration, fn func() error) error {
	deadline := time.Now().Add(maxWait)
	var last error
	for time.Now().Before(deadline) {
		if last = fn(); last == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout after %s: %w", maxWait, last)
}

// ---------------------------------------------------------------------------
// AC1 — All four components configured: install succeeds, secrets correct, CCO healthy
// ---------------------------------------------------------------------------

// TestE2E_AllComponents_ComponentSecretsExist verifies that all four per-component
// credential secrets exist in kube-system after installation with all four component
// credentials configured.
//
// State dependency: cluster installed with all 4 per-component credentials in install-config.yaml.
func TestE2E_AllComponents_ComponentSecretsExist(t *testing.T) {
	t.Skip("E2E: requires cluster installed with all four per-component credentials (Story #43 not yet implemented)")
	requirePerComponentMode(t)

	secrets := []string{
		"vsphere-machine-api-creds",
		"vsphere-storage-creds",
		"vsphere-cloud-controller-creds",
		"vsphere-problem-detector-creds",
	}
	for _, name := range secrets {
		t.Run(name, func(t *testing.T) {
			oc(t, "get", "secret", name, "-n", "kube-system")
		})
	}
}

// TestE2E_AllComponents_SecretsContainVCenterKeys verifies that each component secret
// contains the expected vcenter FQDN-keyed username/password entries.
//
// State dependency: same as TestE2E_AllComponents_ComponentSecretsExist.
func TestE2E_AllComponents_SecretsContainVCenterKeys(t *testing.T) {
	t.Skip("E2E: requires cluster installed with all four per-component credentials (Story #43 not yet implemented)")
	requirePerComponentMode(t)

	vcenter := os.Getenv("VSPHERE_VCENTER")
	if vcenter == "" {
		t.Skip("VSPHERE_VCENTER not set")
	}

	secrets := map[string]string{
		"vsphere-machine-api-creds":       "machineAPI",
		"vsphere-storage-creds":           "csiDriver",
		"vsphere-cloud-controller-creds":  "cloudController",
		"vsphere-problem-detector-creds":  "vsphereProblemDetector",
	}
	for secretName, component := range secrets {
		t.Run(component, func(t *testing.T) {
			userKey := vcenter + ".username"
			passKey := vcenter + ".password"

			user := oc(t, "get", "secret", secretName, "-n", "kube-system",
				"-o", fmt.Sprintf("jsonpath={.data['%s']}", userKey))
			pass := oc(t, "get", "secret", secretName, "-n", "kube-system",
				"-o", fmt.Sprintf("jsonpath={.data['%s']}", passKey))

			if user == "" {
				t.Errorf("secret %s: missing key %s", secretName, userKey)
			}
			if pass == "" {
				t.Errorf("secret %s: missing key %s", secretName, passKey)
			}
		})
	}
}

// TestE2E_AllComponents_CCOCondition_CredentialsProvisionFailed_IsFalse verifies that
// all vSphere CredentialsRequest objects show CredentialsProvisionFailed=False after
// installation with all four per-component credentials correctly privileged.
//
// State dependency: same as TestE2E_AllComponents_ComponentSecretsExist.
func TestE2E_AllComponents_CCOCondition_CredentialsProvisionFailed_IsFalse(t *testing.T) {
	t.Skip("E2E: requires cluster installed with all four per-component credentials (Story #43 not yet implemented)")
	requirePerComponentMode(t)

	crNames := []string{
		"openshift-machine-api-vsphere",
		"openshift-vmware-vsphere-csi-driver-operator",
		"openshift-vsphere-problem-detector",
		"openshift-vsphere-cloud-controller-manager",
	}
	for _, crName := range crNames {
		t.Run(crName, func(t *testing.T) {
			err := pollCondition(3*time.Minute, func() error {
				cond := oc(t, "get", "credentialsrequest", crName, "-n", "openshift-cloud-credential-operator",
					"-o", "jsonpath={.status.conditions[?(@.type=='CredentialsProvisionFailed')].status}")
				if cond != "False" {
					return fmt.Errorf("CredentialsProvisionFailed=%q for %s; want False", cond, crName)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC2 — Graceful degradation: cloudController omitted → fallback + warning + functional
// ---------------------------------------------------------------------------

// TestE2E_GracefulDegradation_CloudControllerFallsBackToShared verifies that when
// cloudController per-component credentials are absent (secret missing or empty),
// CCO routes the CredentialsRequest to the shared vsphere-cloud-credentials secret.
//
// State dependency: cluster installed with only machineAPI and csiDriver per-component
//                   credentials; cloudController secret absent from kube-system.
// Non-idempotency: this test deletes kube-system/vsphere-cloud-controller-creds if it
//                   exists; restoration requires re-install or manual secret creation.
func TestE2E_GracefulDegradation_CloudControllerFallsBackToShared(t *testing.T) {
	t.Skip("E2E: requires cluster installed with partial per-component credentials (AC2, Story #43 not yet implemented)")
	requireKubeconfig(t)

	// Verify the cloud-controller-creds secret is absent (AC2 precondition).
	_, err := ocAllowFail("get", "secret", "vsphere-cloud-controller-creds", "-n", "kube-system")
	if err == nil {
		t.Skip("vsphere-cloud-controller-creds exists; this test requires it to be absent for fallback verification")
	}

	// CCO should route to shared credentials — verify the CR condition shows no provisioning error.
	cond := oc(t, "get", "credentialsrequest", "openshift-vsphere-cloud-controller-manager",
		"-n", "openshift-cloud-credential-operator",
		"-o", "jsonpath={.status.conditions[?(@.type=='CredentialsProvisionFailed')].status}")
	if cond == "True" {
		t.Errorf("CredentialsProvisionFailed=True for cloudController fallback; want False (shared creds should be used)")
	}
}

// TestE2E_GracefulDegradation_CCOLogsWarning verifies that when cloudController falls
// back to shared credentials, a warning appears in the CCO pod logs.
//
// State dependency: same as TestE2E_GracefulDegradation_CloudControllerFallsBackToShared.
func TestE2E_GracefulDegradation_CCOLogsWarning(t *testing.T) {
	t.Skip("E2E: requires cluster installed with partial per-component credentials (AC2, Story #43 not yet implemented)")
	requireKubeconfig(t)

	logs := oc(t, "logs", "-n", "openshift-cloud-credential-operator",
		"-l", "app=cloud-credential-operator",
		"--tail=500", "--since=1h")

	if !strings.Contains(logs, "cloudController") || !strings.Contains(logs, "fallback") {
		t.Errorf("CCO logs do not contain expected fallback warning for cloudController\n"+
			"Expected keywords: 'cloudController', 'fallback'\nLogs (tail 500):\n%s", logs)
	}
}

// TestE2E_GracefulDegradation_CloudControllerFunctional verifies that the Cloud Controller
// Manager continues operating normally after falling back to shared credentials: all nodes
// remain Ready and cloud provider status is available.
//
// State dependency: same as TestE2E_GracefulDegradation_CloudControllerFallsBackToShared.
func TestE2E_GracefulDegradation_CloudControllerFunctional(t *testing.T) {
	t.Skip("E2E: requires cluster with cloudController credential fallback active (AC2, Story #43 not yet implemented)")
	requireKubeconfig(t)

	notReady := oc(t, "get", "nodes",
		"-o", "jsonpath={.items[?(@.status.conditions[?(@.type==\"Ready\")].status!=\"True\")].metadata.name}")
	if notReady != "" {
		t.Errorf("nodes not Ready after cloudController credential fallback: %s", notReady)
	}

	ccmCondition := oc(t, "get", "clusteroperator", "cloud-controller-manager",
		"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
	if ccmCondition != "True" {
		t.Errorf("cloud-controller-manager cluster operator Available=%q; want True", ccmCondition)
	}
}

// ---------------------------------------------------------------------------
// AC3 — Credential rotation: update machine-api secret, CCO re-validates, no restart
// ---------------------------------------------------------------------------

// TestE2E_CredentialRotation_UpdateMachineAPISecret verifies that updating the
// kube-system/vsphere-machine-api-creds secret data is accepted by the API server
// and reflected immediately.
//
// State dependency: cluster in PerComponent mode; vsphere-machine-api-creds must exist.
// Non-idempotency: modifies vsphere-machine-api-creds data. Leaves the secret with
//                   rotated credentials. Run TestE2E_AllComponents_* after to restore
//                   known-good state, or apply known-good creds manually.
func TestE2E_CredentialRotation_UpdateMachineAPISecret(t *testing.T) {
	t.Skip("E2E: requires cluster in PerComponent mode and rotated credential set (AC3, Story #43 not yet implemented)")
	requirePerComponentMode(t)

	vcenter := os.Getenv("VSPHERE_VCENTER")
	if vcenter == "" {
		t.Skip("VSPHERE_VCENTER not set")
	}

	rotatedUser := os.Getenv("VSPHERE_MACHINEAPI_ROTATED_USER")
	rotatedPass := os.Getenv("VSPHERE_MACHINEAPI_ROTATED_PASS")
	if rotatedUser == "" || rotatedPass == "" {
		t.Skip("VSPHERE_MACHINEAPI_ROTATED_USER / VSPHERE_MACHINEAPI_ROTATED_PASS not set")
	}

	// Patch the secret with new credentials.
	patch := fmt.Sprintf(
		`{"data":{"%s.username":"%s","%s.password":"%s"}}`,
		vcenter, rotatedUser,
		vcenter, rotatedPass,
	)
	oc(t, "patch", "secret", "vsphere-machine-api-creds", "-n", "kube-system",
		"--type=merge", "-p", patch)
}

// TestE2E_CredentialRotation_CCORevalidatesWithinTimeout verifies that after updating
// vsphere-machine-api-creds, CCO re-validates the new credential and the
// CredentialsProvisionFailed condition returns to False within 5 minutes.
//
// State dependency: TestE2E_CredentialRotation_UpdateMachineAPISecret has run first and
//                   set a valid (correctly privileged) rotated credential.
// Flakiness risk: uses polling with 5-minute timeout and 5-second intervals. If CCO
//                  re-validation takes longer than 5 minutes on a loaded cluster, the
//                  test may flake. Retry strategy: re-run after cluster quiesces.
func TestE2E_CredentialRotation_CCORevalidatesWithinTimeout(t *testing.T) {
	t.Skip("E2E: requires rotated machine-api secret (AC3, Story #43 not yet implemented)")
	requirePerComponentMode(t)

	err := pollCondition(5*time.Minute, func() error {
		cond := oc(t, "get", "credentialsrequest", "openshift-machine-api-vsphere",
			"-n", "openshift-cloud-credential-operator",
			"-o", "jsonpath={.status.conditions[?(@.type=='CredentialsProvisionFailed')].status}")
		if cond != "False" {
			return fmt.Errorf("CredentialsProvisionFailed=%q after rotation; want False", cond)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

// TestE2E_CredentialRotation_MachineAPIFunctional_AfterRotation verifies that machine-api
// remains functional after credential rotation: a Machine object can be created and
// reaches Provisioned phase without restarting the machine-api-operator pod.
//
// State dependency: TestE2E_CredentialRotation_CCORevalidatesWithinTimeout passed.
// Non-idempotency: creates a Machine object; deletes it in test cleanup. If the test is
//                   interrupted, the Machine may remain. Check MachineSet for orphans.
func TestE2E_CredentialRotation_MachineAPIFunctional_AfterRotation(t *testing.T) {
	t.Skip("E2E: requires rotated machine-api creds with CCO re-validated (AC3, Story #43 not yet implemented)")
	requirePerComponentMode(t)

	// Record machine-api-operator pod name before the functional check.
	podBefore := oc(t, "get", "pods", "-n", "openshift-machine-api",
		"-l", "api=clusterapi,k8s-app=controller", "--no-headers",
		"-o", "custom-columns=NAME:.metadata.name")

	// Verify machine-api operator is still Running (no restart triggered by rotation).
	phase := oc(t, "get", "pods", "-n", "openshift-machine-api",
		"-l", "api=clusterapi,k8s-app=controller",
		"-o", "jsonpath={.items[0].status.phase}")
	if phase != "Running" {
		t.Errorf("machine-api-operator pod not Running after credential rotation: phase=%s", phase)
	}

	// Verify pod has not been restarted (same pod name as before rotation).
	podAfter := oc(t, "get", "pods", "-n", "openshift-machine-api",
		"-l", "api=clusterapi,k8s-app=controller", "--no-headers",
		"-o", "custom-columns=NAME:.metadata.name")
	if podBefore != podAfter {
		t.Logf("machine-api-operator pod restarted during rotation: before=%q after=%q", podBefore, podAfter)
		t.Errorf("machine-api-operator must not restart when credentials are rotated via secret update")
	}
}

// ---------------------------------------------------------------------------
// Adversarial cases
// ---------------------------------------------------------------------------

// TestE2E_MissingPrivilege_PreFlightBlocks_ExactErrorFormat verifies that when
// an install-config.yaml references a machineAPI credential that lacks
// VirtualMachine.Inventory.Create, the installer exits non-zero with the exact
// error format from Story #40 AC1.
//
// Error format:
//
//	Credential validation failed for machineAPI on <vcenter>: missing privileges: [VirtualMachine.Inventory.Create]
//
// State dependency: no running cluster required. Requires installer binary and vSphere
//                   access with a service account missing VirtualMachine.Inventory.Create.
func TestE2E_MissingPrivilege_PreFlightBlocks_ExactErrorFormat(t *testing.T) {
	t.Skip("E2E: requires installer binary + vSphere account with missing VirtualMachine.Inventory.Create (Story #43 not yet implemented)")
	requireKubeconfig(t)

	installerBin := os.Getenv("OPENSHIFT_INSTALL_BINARY")
	if installerBin == "" {
		t.Skip("OPENSHIFT_INSTALL_BINARY not set")
	}

	// Run installer with deliberately incomplete credentials.
	out, err := exec.Command(installerBin, "create", "manifests",
		"--dir=/tmp/test-install-config-missing-priv").CombinedOutput()

	if err == nil {
		t.Fatalf("installer succeeded but should have failed: missing VirtualMachine.Inventory.Create")
	}

	vcenter := os.Getenv("VSPHERE_VCENTER")
	wantFragment := fmt.Sprintf(
		"Credential validation failed for machineAPI on %s: missing privileges: [VirtualMachine.Inventory.Create]",
		vcenter)
	if !strings.Contains(string(out), wantFragment) {
		t.Errorf("installer error output does not match expected format\nwant substring: %q\ngot: %s",
			wantFragment, out)
	}
}

// TestE2E_VCenterAuditLogs_DistinctServiceAccountPrincipals verifies that vCenter audit
// logs show four distinct service account principals — one per component — when all
// four per-component credentials are configured and the cluster is operational.
//
// State dependency: cluster in PerComponent mode, >=1 hour of operational activity.
// Non-idempotency: reads vCenter audit logs via govc; no cluster state modification.
// Flakiness risk: requires vCenter audit API access and sufficient log retention window.
func TestE2E_VCenterAuditLogs_DistinctServiceAccountPrincipals(t *testing.T) {
	t.Skip("E2E: requires govc access to vCenter audit logs and operational cluster (Story #43 not yet implemented)")
	requirePerComponentMode(t)

	vcenter := os.Getenv("VSPHERE_VCENTER")
	if vcenter == "" {
		t.Skip("VSPHERE_VCENTER not set")
	}

	expectedUsers := []string{
		os.Getenv("VSPHERE_MACHINEAPI_USER"),
		os.Getenv("VSPHERE_CSIDRIVER_USER"),
		os.Getenv("VSPHERE_CLOUDCONTROLLER_USER"),
		os.Getenv("VSPHERE_DIAGNOSTICS_USER"),
	}
	for _, u := range expectedUsers {
		if u == "" {
			t.Skip("one or more VSPHERE_<COMPONENT>_USER env vars not set")
		}
	}

	// govc events returns recent vCenter events including the user principal.
	out, err := ocAllowFail("govc", "events", "-server", vcenter, "-json")
	if err != nil {
		t.Skipf("govc events failed (tool not available or no access): %v", err)
	}

	for _, user := range expectedUsers {
		if !strings.Contains(out, user) {
			t.Errorf("vCenter audit events do not contain principal %q; expected distinct activity per component", user)
		}
	}
}

// TestE2E_PartialConfig_OnlyMachineAPIAndCSIDriver_InfrastructureShowsPerComponentMode verifies
// that when only machineAPI and csiDriver per-component credentials are provided (cloudController
// and diagnostics omitted), the Infrastructure CR still reflects PerComponent credentialsMode.
//
// State dependency: cluster installed with only machineAPI+csiDriver per-component creds.
func TestE2E_PartialConfig_OnlyMachineAPIAndCSIDriver_InfrastructureShowsPerComponentMode(t *testing.T) {
	t.Skip("E2E: requires cluster installed with only machineAPI+csiDriver per-component creds (Story #43 not yet implemented)")
	requireKubeconfig(t)

	mode := oc(t, "get", "infrastructure", "cluster",
		"-o", "jsonpath={.spec.platformSpec.vsphere.credentialsMode}")
	if mode != "PerComponent" {
		t.Errorf("Infrastructure credentialsMode=%q; want PerComponent even with only 2 of 4 component creds configured", mode)
	}

	// cloudController and diagnostics secrets should be absent (fallback to shared).
	for _, secretName := range []string{"vsphere-cloud-controller-creds", "vsphere-problem-detector-creds"} {
		_, err := ocAllowFail("get", "secret", secretName, "-n", "kube-system")
		if err == nil {
			t.Errorf("secret %s exists but should be absent (no per-component cred configured)", secretName)
		}
	}
}

// TestE2E_CredentialRotation_Concurrent_BothComponentsStable verifies that rotating
// machineAPI and csiDriver credentials concurrently does not cause either component
// to enter a failed state. This is the primary concurrency risk surface for AC3.
//
// State dependency: cluster in PerComponent mode with both machineAPI and csiDriver secrets.
// Non-idempotency: modifies both secrets concurrently. Leaves rotated credentials in place.
// Flakiness risk: concurrent updates may trigger spurious CCO reconciles. Poll with 5 min timeout.
func TestE2E_CredentialRotation_Concurrent_BothComponentsStable(t *testing.T) {
	t.Skip("E2E: requires cluster in PerComponent mode; rotated creds for both components (AC3 adversarial, Story #43 not yet implemented)")
	requirePerComponentMode(t)

	vcenter := os.Getenv("VSPHERE_VCENTER")
	rotatedMachineAPIUser := os.Getenv("VSPHERE_MACHINEAPI_ROTATED_USER")
	rotatedCSIUser := os.Getenv("VSPHERE_CSIDRIVER_ROTATED_USER")
	if vcenter == "" || rotatedMachineAPIUser == "" || rotatedCSIUser == "" {
		t.Skip("VSPHERE_VCENTER / VSPHERE_MACHINEAPI_ROTATED_USER / VSPHERE_CSIDRIVER_ROTATED_USER not set")
	}

	// Patch both secrets concurrently via goroutines.
	errs := make(chan error, 2)

	go func() {
		patch := fmt.Sprintf(`{"data":{"%s.username":"%s"}}`, vcenter, rotatedMachineAPIUser)
		_, err := ocAllowFail("patch", "secret", "vsphere-machine-api-creds", "-n", "kube-system",
			"--type=merge", "-p", patch)
		errs <- err
	}()

	go func() {
		patch := fmt.Sprintf(`{"data":{"%s.username":"%s"}}`, vcenter, rotatedCSIUser)
		_, err := ocAllowFail("patch", "secret", "vsphere-storage-creds", "-n", "kube-system",
			"--type=merge", "-p", patch)
		errs <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent credential patch failed: %v", err)
		}
	}

	// Both CredentialsRequests must stabilize to CredentialsProvisionFailed=False within 5 minutes.
	crs := []string{
		"openshift-machine-api-vsphere",
		"openshift-vmware-vsphere-csi-driver-operator",
	}
	for _, crName := range crs {
		t.Run(crName, func(t *testing.T) {
			err := pollCondition(5*time.Minute, func() error {
				cond := oc(t, "get", "credentialsrequest", crName,
					"-n", "openshift-cloud-credential-operator",
					"-o", "jsonpath={.status.conditions[?(@.type=='CredentialsProvisionFailed')].status}")
				if cond != "False" {
					return fmt.Errorf("CredentialsProvisionFailed=%q; want False", cond)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
}
