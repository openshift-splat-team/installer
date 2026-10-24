package hooks

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	externaltypes "github.com/openshift/installer/pkg/types/external"
)

// writeHook places a script at the given path relative to the External
// manifest directory and returns the install directory.
//
// The hooks really are executed. The contract this package defines is a
// process contract -- environment variables, a working directory, an exit
// status -- and none of that is exercised by a fake. The scripts are trivial
// and POSIX, so the cost is a skip on platforms without a shell.
func writeHook(t *testing.T, rel, body string, mode os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, externaltypes.ManifestDir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), mode))
	return dir
}

// TestRunAcceptsARelativeInstallDir is a regression test for a real install
// failure: `openshift-install create cluster --dir install-dir-mrb-ext11` died
// with
//
//	failed to run the infra-ready hook hooks/infra-hook.sh:
//	fork/exec install-dir-mrb-ext11/external-install/hooks/infra-hook.sh:
//	no such file or directory
//
// one line after the installer had opened that same file and logged its
// SHA-256. A relative --dir is the ordinary way to invoke the installer, and
// every test above used t.TempDir(), which is absolute -- so the whole suite
// passed while the only thing anyone actually runs was broken.
//
// The cause is that cmd.Dir makes the child chdir before it execs, so a
// relative program path is resolved against the install directory a second
// time. The assertion on OPENSHIFT_INSTALL_STATE_DIR guards the quieter half
// of the same bug: a relative state directory would resolve against the
// hook's working directory, and a hook that creates it before writing would
// leave its state where pre-destroy will never look.
func TestRunAcceptsARelativeInstallDir(t *testing.T) {
	skipWithoutShell(t)

	t.Chdir(t.TempDir())
	rel := "install-dir-relative"
	path := filepath.Join(rel, externaltypes.ManifestDir, "hooks/probe.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	// Executable on purpose: the bug under test is an exec failure, so a hook
	// that could not have run anyway would make this pass for the wrong reason.
	//nolint:gosec // G306: a hook has to be runnable to test how it is run.
	require.NoError(t, os.WriteFile(path,
		[]byte("#!/bin/sh\nprintf '%s' \"$OPENSHIFT_INSTALL_STATE_DIR\" > \"$OPENSHIFT_INSTALL_STATE_DIR/seen\"\n"),
		0o755))

	require.NoError(t, Run(context.Background(), Request{
		Kind: InfraReady, Program: "hooks/probe.sh", InstallDir: rel,
	}))

	// Written by the hook itself, so it can only exist if the hook both ran
	// and resolved the state directory to the place the installer meant.
	seen, err := os.ReadFile(filepath.Join(rel, externaltypes.ManifestDir, externaltypes.HookStateDir, "seen"))
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(string(seen)),
		"the hook must receive an absolute OPENSHIFT_INSTALL_STATE_DIR, got %q", string(seen))
}

func skipWithoutShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the hook scripts are POSIX shell")
	}
}

// TestRunPassesTheContract is the test that matters most: the environment is
// the published interface between the installer and a partner's automation,
// so every variable a hook is documented to receive is asserted by name.
// Renaming one silently would break every hook already written against it.
func TestRunPassesTheContract(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/dump.sh",
		"#!/bin/sh\nenv | grep '^OPENSHIFT_INSTALL_' | sort > \"$OPENSHIFT_INSTALL_STATE_DIR/env\"\npwd >> \"$OPENSHIFT_INSTALL_STATE_DIR/env\"\n",
		0o755)

	require.NoError(t, Run(context.Background(), Request{
		Kind:                     InfraReady,
		Program:                  "hooks/dump.sh",
		InstallDir:               dir,
		InfraID:                  "example-abcde",
		ClusterName:              "example",
		BaseDomain:               "test.example.com",
		Publish:                  "External",
		ControlPlaneEndpointHost: "lb.example.internal",
		ControlPlaneEndpointPort: "6443",
		ClusterJSON:              []byte(`{"kind":"Cluster"}`),
		InfraJSON:                []byte(`{"kind":"SomeProviderCluster"}`),
	}))

	stateDir := filepath.Join(dir, externaltypes.ManifestDir, externaltypes.HookStateDir)
	out, err := os.ReadFile(filepath.Join(stateDir, "env"))
	require.NoError(t, err)
	env := string(out)

	for _, want := range []string{
		"OPENSHIFT_INSTALL_HOOK=infra-ready",
		"OPENSHIFT_INSTALL_INFRA_ID=example-abcde",
		"OPENSHIFT_INSTALL_CLUSTER_NAME=example",
		"OPENSHIFT_INSTALL_BASE_DOMAIN=test.example.com",
		// The join, not a second field to keep in step with the other two.
		"OPENSHIFT_INSTALL_CLUSTER_DOMAIN=example.test.example.com",
		"OPENSHIFT_INSTALL_PUBLISH=External",
		"OPENSHIFT_INSTALL_DIR=" + dir,
		"OPENSHIFT_INSTALL_MANIFEST_DIR=" + filepath.Join(dir, externaltypes.ManifestDir),
		"OPENSHIFT_INSTALL_STATE_DIR=" + stateDir,
		"OPENSHIFT_INSTALL_CONTROL_PLANE_ENDPOINT_HOST=lb.example.internal",
		"OPENSHIFT_INSTALL_CONTROL_PLANE_ENDPOINT_PORT=6443",
	} {
		assert.Contains(t, env, want)
	}

	// The working directory is the install directory, so a hook can use
	// paths relative to it the way the user thinks about them.
	assert.Contains(t, env, dir)
}

// TestRunPassesObjectsAsReadableFiles covers the mechanism that makes a hook
// able to do anything provider-specific at all: the provider's own object,
// which the installer has no type for, handed over as JSON.
func TestRunPassesObjectsAsReadableFiles(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/copy.sh",
		"#!/bin/sh\ncat \"$OPENSHIFT_INSTALL_INFRA_JSON\" > \"$OPENSHIFT_INSTALL_STATE_DIR/infra\"\n"+
			"cat \"$OPENSHIFT_INSTALL_CLUSTER_JSON\" > \"$OPENSHIFT_INSTALL_STATE_DIR/cluster\"\n",
		0o755)

	infra := []byte(`{"kind":"AWSCluster","status":{"networkStatus":{"apiServerElb":{"dnsName":"lb.internal"}}}}`)
	require.NoError(t, Run(context.Background(), Request{
		Kind: InfraReady, Program: "hooks/copy.sh", InstallDir: dir,
		ClusterJSON: []byte(`{"kind":"Cluster"}`), InfraJSON: infra,
	}))

	stateDir := filepath.Join(dir, externaltypes.ManifestDir, externaltypes.HookStateDir)
	got, err := os.ReadFile(filepath.Join(stateDir, "infra"))
	require.NoError(t, err)
	assert.JSONEq(t, string(infra), string(got))

	got, err = os.ReadFile(filepath.Join(stateDir, "cluster"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"Cluster"}`, string(got))
}

// TestRunOmitsUnavailableObjects checks that an absent object is an unset
// variable rather than an empty file. A hook testing -f on the path would be
// misled by an empty file into reading an object that is not there.
func TestRunOmitsUnavailableObjects(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/probe.sh",
		"#!/bin/sh\nprintf '%s' \"${OPENSHIFT_INSTALL_INFRA_JSON-unset}\" > \"$OPENSHIFT_INSTALL_STATE_DIR/probe\"\n",
		0o755)

	require.NoError(t, Run(context.Background(), Request{
		Kind: InfraReady, Program: "hooks/probe.sh", InstallDir: dir,
	}))

	got, err := os.ReadFile(filepath.Join(dir, externaltypes.ManifestDir, externaltypes.HookStateDir, "probe"))
	require.NoError(t, err)
	assert.Equal(t, "unset", string(got))
}

// TestStateDirSurvivesBetweenHooks is the property destroy coverage rests on.
// What infra-ready records is what pre-destroy reads, and if that did not
// survive, resources created outside Cluster API's ownership would leak with
// nothing left to identify them.
func TestStateDirSurvivesBetweenHooks(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/both.sh",
		"#!/bin/sh\n"+
			"if [ \"$OPENSHIFT_INSTALL_HOOK\" = infra-ready ]; then echo zone-123 > \"$OPENSHIFT_INSTALL_STATE_DIR/dns\"; "+
			"else cp \"$OPENSHIFT_INSTALL_STATE_DIR/dns\" \"$OPENSHIFT_INSTALL_STATE_DIR/seen\"; fi\n",
		0o755)

	ctx := context.Background()
	require.NoError(t, Run(ctx, Request{Kind: InfraReady, Program: "hooks/both.sh", InstallDir: dir}))
	require.NoError(t, Run(ctx, Request{Kind: PreDestroy, Program: "hooks/both.sh", InstallDir: dir}))

	got, err := os.ReadFile(filepath.Join(dir, externaltypes.ManifestDir, externaltypes.HookStateDir, "seen"))
	require.NoError(t, err)
	assert.Equal(t, "zone-123\n", string(got))
}

// TestRunPassesArgsVerbatim is the flag half of the contract: the installer
// hands the program its argv unchanged, which is what lets a hook be a
// compiled binary as readily as a script and lets a failed hook be rerun by
// hand from the command line in the installer's log.
func TestRunPassesArgsVerbatim(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/args.sh",
		"#!/bin/sh\nfor a in \"$@\"; do echo \"[$a]\" >>\"${OPENSHIFT_INSTALL_STATE_DIR}/argv\"; done\n", 0o755)

	// A value with a space in it is the case worth pinning: it survives only
	// if the installer never puts the argv through a shell.
	args := []string{"--input-service=openshift-ingress/router-external-default", "--input-dns-zone", "Z 1"}
	require.NoError(t, Run(context.Background(), Request{
		Kind: PostProvision, Program: "hooks/args.sh", Args: args, InstallDir: dir,
	}))

	got, err := os.ReadFile(filepath.Join(dir, externaltypes.ManifestDir, externaltypes.HookStateDir, "argv"))
	require.NoError(t, err)
	assert.Equal(t, "[--input-service=openshift-ingress/router-external-default]\n[--input-dns-zone]\n[Z 1]\n", string(got))
}

// TestRunRejects covers every way a hook can be unusable. Each of these is a
// failure the user can fix, so each error has to name the thing to fix.
func TestRunRejects(t *testing.T) {
	skipWithoutShell(t)

	t.Run("non-zero exit fails the install", func(t *testing.T) {
		dir := writeHook(t, "hooks/fail.sh", "#!/bin/sh\necho breaking >&2\nexit 3\n", 0o755)
		err := Run(context.Background(), Request{Kind: InfraReady, Program: "hooks/fail.sh", InstallDir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exited 3")
		// The hook's own last words, not a pointer to the log. A hook is the
		// only thing that knows why it failed.
		assert.Contains(t, err.Error(), "breaking")
	})

	t.Run("a hook that says nothing says so", func(t *testing.T) {
		dir := writeHook(t, "hooks/quiet.sh", "#!/bin/sh\nexit 1\n", 0o755)
		err := Run(context.Background(), Request{Kind: InfraReady, Program: "hooks/quiet.sh", InstallDir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "produced no output")
	})

	t.Run("missing program", func(t *testing.T) {
		dir := writeHook(t, "hooks/present.sh", "#!/bin/sh\n", 0o755)
		err := Run(context.Background(), Request{Kind: InfraReady, Program: "hooks/absent.sh", InstallDir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("not executable says how to fix it", func(t *testing.T) {
		dir := writeHook(t, "hooks/plain.sh", "#!/bin/sh\n", 0o644)
		err := Run(context.Background(), Request{Kind: InfraReady, Program: "hooks/plain.sh", InstallDir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chmod +x")
	})

	t.Run("a directory is not a program", func(t *testing.T) {
		dir := writeHook(t, "hooks/x.sh", "#!/bin/sh\n", 0o755)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, externaltypes.ManifestDir, "hooks", "sub"), 0o755))
		err := Run(context.Background(), Request{Kind: InfraReady, Program: "hooks/sub", InstallDir: dir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory")
	})

	// Containment is what makes the install directory a self-contained unit
	// that can be archived and handed on. A hook reaching outside it would
	// run something that did not travel with it.
	for _, escape := range []string{"../escape.sh", "hooks/../../escape.sh", "/bin/sh"} {
		t.Run("refuses "+escape, func(t *testing.T) {
			dir := writeHook(t, "hooks/x.sh", "#!/bin/sh\n", 0o755)
			// Executable on purpose: a target that could not have run anyway
			// would let this pass without containment being what stopped it.
			//nolint:gosec // G306: the escape target has to be runnable to be a real target.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "escape.sh"), []byte("#!/bin/sh\n"), 0o755))
			err := Run(context.Background(), Request{Kind: InfraReady, Program: escape, InstallDir: dir})
			require.Error(t, err)
			assert.True(t,
				strings.Contains(err.Error(), "relative path inside"),
				"expected a containment error, got: %v", err)
		})
	}

	t.Run("no program configured", func(t *testing.T) {
		err := Run(context.Background(), Request{Kind: InfraReady, InstallDir: t.TempDir()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no hook program configured")
	})
}

// TestRunCancels checks that a hanging hook is bounded by the caller's
// context rather than hanging the install indefinitely.
func TestRunCancels(t *testing.T) {
	skipWithoutShell(t)

	dir := writeHook(t, "hooks/sleep.sh", "#!/bin/sh\nsleep 60\n", 0o755)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, Request{Kind: InfraReady, Program: "hooks/sleep.sh", InstallDir: dir})
	assert.Error(t, err)
}
