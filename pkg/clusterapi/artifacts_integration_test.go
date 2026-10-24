package clusterapi

import (
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/installer/cmd/openshift-install/command"
)

// The tests in this file stand up the installer's local Cluster API management
// cluster -- an envtest etcd and kube-apiserver started on this host -- and run
// an infrastructure controller against it. They are the only place the artifact
// override is exercised end to end rather than unit tested.
//
// Two things this does not do, and must not be read as doing. It makes no cloud
// call and needs no credentials: it stops at the local control plane. And
// nothing it creates -- CRDs, webhook configurations, anything else -- reaches a
// cluster being installed; the local control plane is temporary and is torn down
// when the test ends.
//
// Why the pieces are assembled here instead of calling system.Run: that path
// loads metadata.json from command.RootOpts.Dir (system.go:176), which a test
// would have to fabricate, and unpacks the component manifests from data.Assets
// (system.go:155), which data/assets.go:16-20 binds in an init() that has
// already run before TestMain -- so t.Setenv("OPENSHIFT_INSTALL_DATA", ...)
// cannot reach it, and the dev-build default http.Dir("data") resolves against
// the test's working directory to a nonexistent pkg/clusterapi/data.
//
// runController reads only c.lcp.{BinDir,Cfg,Env.Scheme,KubeconfigPath} and
// c.logWriter; getInfrastructureController reads only c.componentDir and
// c.lcp.BinDir (system.go:577-593, :615-754). Those are the fields populated
// below.
//
// Selection is by name: hack/go-integration-test.sh:4 runs -run .Integration,
// there is no build tag. hack/go-test.sh passes -short, which these skip on.
// Neither test may call t.Parallel: both mutate process-wide state
// (command.RootOpts.Dir, and the override environment variables).

const (
	// capaComponentsFile is the component manifest getInfrastructureController
	// looks for in componentDir, by the convention at system.go:579.
	capaComponentsFile = "aws-infrastructure-components.yaml"

	// capaComponentsSource is the in-tree copy of that manifest. data.Unpack
	// would normally place it in componentDir; see the note above on why this
	// test cannot use data.Unpack.
	capaComponentsSource = "../../data/data/cluster-api/" + capaComponentsFile

	// capaCRDName is one of the CRDs the manifest carries, used to prove the
	// CRDs reached the local control plane.
	capaCRDName = "awsclusters.infrastructure.cluster.x-k8s.io"

	// capaMutatingWebhookName and capaValidatingWebhookName are the webhook
	// configurations the manifest carries. envtest rewrites their in-cluster
	// Service references to a local URL before installing them.
	capaMutatingWebhookName   = "capa-mutating-webhook-configuration"
	capaValidatingWebhookName = "capa-validating-webhook-configuration"

	// capaBinaryName is the file Provider.Extract writes for the AWS provider.
	// AWS.Sources is exactly this one name (providers.go:68-75), and
	// unpackFile (providers.go:136-151) is the only thing that writes it, so
	// its presence or absence in BinDir is a direct read of whether Extract
	// ran.
	capaBinaryName = "cluster-api-provider-aws"
)

// capaArgs is the argument list system.go:192-199 gives the AWS provider,
// minus the flags a particular arm of the test does not want. It is spelled
// out rather than imported so that a change to the production list shows up
// here as a deliberate edit.
func capaArgs(healthAddr bool) []string {
	args := []string{
		"-v=4",
		"--diagnostics-address=0",
	}
	if healthAddr {
		args = append(args, "--health-addr={{suggestHealthHostPort}}")
	}
	return append(args,
		"--webhook-port={{.WebhookPort}}",
		"--webhook-cert-dir={{.WebhookCertDir}}",
		"--feature-gates=BootstrapFormatIgnition=true,ExternalResourceGC=true,TagUnmanagedNetworkResources=false,EKS=false,MachinePool=false",
	)
}

// skipUnlessIntegration skips under -short, and skips when the embedded mirror
// is absent. The mirror is checked through Mirror.Open rather than by stat'ing
// a path on disk because that is exactly what localControlPlane.Run and
// Provider.Extract read (providers.go:84, localcontrolplane.go:71-76): in a
// release build the zip exists only inside the binary, and in a fresh clone
// mirror/ holds nothing but a README. A missing artifact is a skip, not a
// failure.
func skipUnlessIntegration(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test: -short was requested")
	}

	f, err := Mirror.Open(path.Join("mirror", zipFile))
	if err != nil {
		t.Skipf("skipping integration test: the embedded Cluster API mirror is not present (%v); "+
			"build it with ./hack/build.sh", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close the embedded Cluster API mirror: %v", err)
	}
}

// newLocalSystem starts a local control plane and returns a system wired to it,
// populated with just the fields getInfrastructureController and runController
// read. Everything it creates lives under t.TempDir and is stopped on cleanup.
func newLocalSystem(t *testing.T) *system {
	t.Helper()

	// command.RootOpts.Dir drives BinDir (localcontrolplane.go:70), the etcd
	// data directory (:77) and the etcd and kube-apiserver log files (:84-88).
	// It is a package-level variable, so it is saved and restored rather than
	// merely set.
	savedDir := command.RootOpts.Dir
	command.RootOpts.Dir = t.TempDir()
	t.Cleanup(func() { command.RootOpts.Dir = savedDir })

	lcp := &localControlPlane{}
	if err := lcp.Run(t.Context()); err != nil {
		t.Fatalf("failed to start the local control plane: %v", err)
	}
	t.Cleanup(func() {
		if err := lcp.Stop(); err != nil {
			t.Errorf("failed to stop the local control plane: %v", err)
		}
		if err := lcp.EtcdLog.Close(); err != nil {
			t.Errorf("failed to close the etcd log: %v", err)
		}
		if err := lcp.APIServerLog.Close(); err != nil {
			t.Errorf("failed to close the kube-apiserver log: %v", err)
		}
	})

	logWriter := logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		if err := logWriter.Close(); err != nil {
			t.Errorf("failed to close the controller log writer: %v", err)
		}
	})

	return &system{
		lcp:          lcp,
		client:       lcp.Client,
		componentDir: newComponentDir(t),
		logWriter:    logWriter,
	}
}

// newComponentDir returns a directory holding the AWS component manifest under
// the name getInfrastructureController expects, standing in for the directory
// data.Unpack would have produced.
func newComponentDir(t *testing.T) string {
	t.Helper()

	manifest, err := os.ReadFile(capaComponentsSource)
	if err != nil {
		t.Fatalf("failed to read the AWS component manifest: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, capaComponentsFile), manifest, 0o600); err != nil {
		t.Fatalf("failed to stage the AWS component manifest: %v", err)
	}
	return dir
}

// clearArtifactOverride neutralises any override the developer running the test
// happens to have exported, so that a test asserting on the default path is
// asserting on the default path. lookupArtifactOverride treats an empty value
// as unset (artifacts.go:56-60).
func clearArtifactOverride(t *testing.T) {
	t.Helper()

	binaryEnv, componentsEnv := artifactEnvNames(AWS.Name)
	t.Setenv(binaryEnv, "")
	t.Setenv(componentsEnv, "")
}

// writeStubController writes a controller stand-in that starts, survives, and
// exits on SIGTERM. It is not an ELF, which validateHostArch deliberately
// accepts (artifacts.go:201-212) so that the override stays usable on a
// developer workstation. Using it keeps this arm fast and free of any need for
// cloud credentials: what is under test is the seam, not CAPA.
func writeStubController(t *testing.T) string {
	t.Helper()

	stub := filepath.Join(t.TempDir(), "stub-cluster-api-provider-aws")
	// The stub ignores its arguments. exec replaces the shell, so the process
	// the installer started is the process that receives SIGTERM from
	// process.State.Stop.
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexec sleep 300\n"), 0o600); err != nil {
		t.Fatalf("failed to write the stub controller: %v", err)
	}
	// #nosec G302 -- a controller binary has to carry the execute bit, and
	// validateExecutable (artifacts.go:167-169) rejects it otherwise. 0o700 is
	// the narrowest mode that satisfies both.
	if err := os.Chmod(stub, 0o700); err != nil {
		t.Fatalf("failed to make the stub controller executable: %v", err)
	}
	return stub
}

// assertComponentsInstalled checks that the provider's manifest reached the
// local control plane: its CRDs exist, and its webhook configurations were
// rewritten by envtest from an in-cluster Service reference to a local URL.
// Neither of these is delivered anywhere else -- they live and die with the
// envtest API server.
func assertComponentsInstalled(t *testing.T, cl client.Client) {
	t.Helper()

	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	assert.NoError(t, cl.Get(t.Context(), client.ObjectKey{Name: capaCRDName}, crd),
		"the provider CRDs must be installed in the local control plane")

	mutating := &admissionv1.MutatingWebhookConfiguration{}
	if assert.NoError(t, cl.Get(t.Context(), client.ObjectKey{Name: capaMutatingWebhookName}, mutating),
		"the mutating webhook configuration must be installed in the local control plane") {
		assert.NotEmpty(t, mutating.Webhooks, "the mutating webhook configuration carries no webhooks")
		for _, wh := range mutating.Webhooks {
			assertWebhookRewritten(t, wh.Name, wh.ClientConfig)
		}
	}

	validating := &admissionv1.ValidatingWebhookConfiguration{}
	if assert.NoError(t, cl.Get(t.Context(), client.ObjectKey{Name: capaValidatingWebhookName}, validating),
		"the validating webhook configuration must be installed in the local control plane") {
		assert.NotEmpty(t, validating.Webhooks, "the validating webhook configuration carries no webhooks")
		for _, wh := range validating.Webhooks {
			assertWebhookRewritten(t, wh.Name, wh.ClientConfig)
		}
	}
}

// assertWebhookRewritten checks one webhook's client config: the in-cluster
// Service is gone, the replacement URL points at a loopback address on this
// host, and a CA bundle for envtest's self-signed serving certificate is
// present (vendor/sigs.k8s.io/controller-runtime/pkg/envtest/webhook.go:110-117).
func assertWebhookRewritten(t *testing.T, name string, cc admissionv1.WebhookClientConfig) {
	t.Helper()

	assert.Nil(t, cc.Service, "webhook %q still points at an in-cluster Service", name)
	assert.NotEmpty(t, cc.CABundle, "webhook %q has no CA bundle for the local serving certificate", name)
	if !assert.NotNil(t, cc.URL, "webhook %q was not rewritten to a local URL", name) {
		return
	}

	parsed, err := url.Parse(*cc.URL)
	if !assert.NoError(t, err, "webhook %q has an unparseable URL", name) {
		return
	}
	assert.Equal(t, "https", parsed.Scheme, "webhook %q is not served over TLS", name)
	host, _, err := net.SplitHostPort(parsed.Host)
	if !assert.NoError(t, err, "webhook %q has no host:port", name) {
		return
	}
	ip := net.ParseIP(host)
	if assert.NotNil(t, ip, "webhook %q host %q is not an IP address", name, host) {
		assert.True(t, ip.IsLoopback(),
			"webhook %q points at %s, which is not on this host", name, host)
	}
}

// stopController stops the controller and reports whether the process is gone.
// process.State.Stop sends SIGTERM and waits for the child (process_unix.go:18,
// process.go:227-253); the exit error it records is the signal, so only the
// fact of exit is asserted.
func stopController(t *testing.T, ct *controller) {
	t.Helper()

	if ct.state == nil {
		return
	}
	assert.NoError(t, ct.state.Stop(), "the controller did not stop on request")
	exited, exitErr := ct.state.Exited()
	assert.True(t, exited, "the controller process outlived the test (exit error: %v)", exitErr)
}

// TestRunControllerArtifactOverrideIntegration is the proof the pilot rests on:
// a developer-supplied controller binary is started, by the installer's own
// startup path, against the local control plane -- and the embedded copy is
// never unpacked.
//
// The argument list deliberately omits {{suggestHealthHostPort}}, so
// healthCheckHostPort stays empty, process.State.HealthCheck stays nil, and
// Start returns as soon as the fork succeeds
// (system.go:732-740, internal/process/process.go:141-146). That is the whole
// claim being tested: the override reaches exec. Waiting on a health endpoint
// would only test the stub's ability to serve one.
func TestRunControllerArtifactOverrideIntegration(t *testing.T) {
	skipUnlessIntegration(t)

	c := newLocalSystem(t)

	clearArtifactOverride(t)
	stub := writeStubController(t)
	binaryEnv, _ := artifactEnvNames(AWS.Name)
	t.Setenv(binaryEnv, stub)

	ct := c.getInfrastructureController(&AWS, capaArgs(false), map[string]string{})

	// The path the controller would take without the override, and the file
	// Extract would have written.
	embedded := filepath.Join(c.lcp.BinDir, capaBinaryName)
	assert.Equal(t, embedded, ct.Path,
		"the controller must default to the embedded binary before the override is applied")
	assert.Len(t, ct.Components, 1,
		"the staged component manifest must have been found in componentDir")

	if !assert.NoError(t, c.runController(t.Context(), ct)) {
		return
	}
	t.Cleanup(func() {
		if ct.state == nil {
			return
		}
		if err := ct.state.Stop(); err != nil {
			t.Logf("failed to stop the controller during cleanup: %v", err)
		}
	})

	// The override was applied, and it is what ran.
	assert.True(t, ct.skipExtract, "the override must suppress extraction")
	assert.Equal(t, stub, ct.Path, "the override must replace ct.Path")
	if assert.NotNil(t, ct.state) && assert.NotNil(t, ct.state.Cmd) {
		assert.Equal(t, stub, ct.state.Cmd.Path,
			"the process must be started from the override path, not the embedded binary")
	}

	// Extract was not reached. AWS.Sources is exactly {cluster-api-provider-aws}
	// (providers.go:68-75) and Extract is the only writer of that name in
	// BinDir (providers.go:117-131), so its absence is the observable proof --
	// no log string needed.
	_, err := os.Stat(embedded)
	assert.True(t, os.IsNotExist(err),
		"%s exists, so Provider.Extract ran despite skipExtract (stat error: %v)", embedded, err)

	// The component manifest was not overridden, so the embedded one is what
	// reached the local control plane.
	assertComponentsInstalled(t, c.client)

	stopController(t, ct)
}

// TestRunControllerEmbeddedBinaryIntegration is the regression guarantee. With
// no override configured, the controller must still come from the embedded
// mirror and still start -- this is the evidence that the seam changes nothing
// when it is not used, which the unit tests cannot give because they never
// reach runController.
//
// Unlike the override arm this one uses the real CAPA binary, because the claim
// is precisely that the embedded path is intact.
func TestRunControllerEmbeddedBinaryIntegration(t *testing.T) {
	skipUnlessIntegration(t)

	c := newLocalSystem(t)
	clearArtifactOverride(t)

	embedded := filepath.Join(c.lcp.BinDir, capaBinaryName)
	_, err := os.Stat(embedded)
	assert.True(t, os.IsNotExist(err),
		"precondition: %s must not exist before runController extracts it (stat error: %v)", embedded, err)

	ct := c.getInfrastructureController(&AWS, capaArgs(true), map[string]string{})
	assert.Equal(t, embedded, ct.Path,
		"with no override the controller must point at the embedded binary")

	if !assert.NoError(t, c.runController(t.Context(), ct)) {
		return
	}
	t.Cleanup(func() {
		if ct.state == nil {
			return
		}
		if err := ct.state.Stop(); err != nil {
			t.Logf("failed to stop the controller during cleanup: %v", err)
		}
	})

	assert.False(t, ct.skipExtract, "with no override the embedded binary must still be extracted")
	assert.FileExists(t, embedded, "Provider.Extract did not unpack the embedded controller")
	if assert.NotNil(t, ct.state) && assert.NotNil(t, ct.state.Cmd) {
		assert.Equal(t, embedded, ct.state.Cmd.Path,
			"the process must be started from the extracted embedded binary")
	}

	// capaArgs(true) asked for {{suggestHealthHostPort}}, so runController
	// configured a health check and Start only returned once /healthz answered
	// 200 (system.go:732-740). Assert the wiring that gating depended on, so a
	// silent regression to an ungated start is visible.
	if assert.NotNil(t, ct.state) {
		if assert.NotNil(t, ct.state.HealthCheck, "the health check must have been configured") {
			assert.Equal(t, "/healthz", ct.state.HealthCheck.Path,
				"the health check must poll /healthz")
		}
	}
	assert.True(t, hasRenderedHealthAddr(ct.Args),
		"--health-addr was not rendered to a concrete host:port: %v", ct.Args)

	assertComponentsInstalled(t, c.client)

	stopController(t, ct)
}

// hasRenderedHealthAddr reports whether the rendered argument list carries a
// --health-addr with a concrete host:port rather than the template.
func hasRenderedHealthAddr(args []string) bool {
	const flag = "--health-addr="
	for _, arg := range args {
		if !strings.HasPrefix(arg, flag) {
			continue
		}
		value := strings.TrimPrefix(arg, flag)
		if strings.Contains(value, "{{") {
			return false
		}
		if _, _, err := net.SplitHostPort(value); err != nil {
			return false
		}
		return true
	}
	return false
}
