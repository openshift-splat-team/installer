package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/installer/pkg/clusterapi"
)

// TestExtractCmdIsHidden pins the reason this command exists at all. It writes
// artifacts that only the External platform consumes, and External is not a
// supported configuration yet, so it must not appear in `openshift-install
// --help` alongside commands users are meant to run.
func TestExtractCmdIsHidden(t *testing.T) {
	cmd := newExtractCmd()
	assert.True(t, cmd.Hidden, "extract must stay hidden until the External platform is supported")

	sub, _, err := cmd.Find([]string{"cluster-api"})
	require.NoError(t, err)
	assert.Equal(t, "cluster-api", sub.Name())
}

func TestExtractClusterAPICmdArgs(t *testing.T) {
	cmd := newExtractClusterAPICmd()

	for _, args := range [][]string{{}, {"aws", "azure"}} {
		err := cmd.Args(cmd, args)
		require.Error(t, err, "args %v", args)
		// The root command silences usage (main.go:79-80), so this message is
		// all the caller gets. It has to name the providers, because which
		// ones a given binary embeds is not something the caller can know.
		assert.Contains(t, err.Error(), clusterapi.AWS.Name)
	}

	assert.NoError(t, cmd.Args(cmd, []string{"aws"}))
}

// TestExtractClusterAPICmdRequiresDestDir covers the input that would otherwise
// write a 150MB binary into whatever directory the caller happened to be in.
func TestExtractClusterAPICmdRequiresDestDir(t *testing.T) {
	// extractOpts is package-level state shared with the real command, so this
	// test restores it and must not run in parallel.
	saved := extractOpts.destDir
	t.Cleanup(func() { extractOpts.destDir = saved })

	// Building the command binds the flag, which resets destDir; set the value
	// afterwards so the test is not at the mercy of that ordering.
	cmd := newExtractClusterAPICmd()
	extractOpts.destDir = ""

	err := cmd.RunE(cmd, []string{clusterapi.AWS.Name})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--dest-dir")
}

// TestExtractClusterAPICmdLongListsProviders keeps the help text honest: it is
// generated from the same list the command validates against, so a provider
// added to one cannot go missing from the other.
func TestExtractClusterAPICmdLongListsProviders(t *testing.T) {
	long := newExtractClusterAPICmd().Long
	for _, name := range clusterapi.InfrastructureProviderNames() {
		assert.Contains(t, long, name)
	}
	assert.NotContains(t, strings.ToLower(long), clusterapi.EnvTest.Name,
		"envtest is unpacked by the installer itself and must not be offered")
}
