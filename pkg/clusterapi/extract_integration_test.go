package clusterapi

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/installer/data"
)

// TestExtractProviderIntegration extracts a real provider out of the embedded
// artifacts. It is an integration test only because those artifacts have to
// have been built by hack/build.sh; it makes no cloud call, starts nothing and
// needs no credentials.
//
// There is a second reason it cannot be a unit test. data.Assets is bound in an
// init() to http.Dir("data") (data/assets.go:16-20), which resolves against the
// test's working directory to a nonexistent pkg/clusterapi/data, and the init()
// has already run by the time any test could set OPENSHIFT_INSTALL_DATA. The
// binding is replaced below instead. That is process-wide state, so this test
// must not call t.Parallel.
func TestExtractProviderIntegration(t *testing.T) {
	skipUnlessIntegration(t)

	savedAssets := data.Assets
	data.Assets = http.Dir("../../data/data")
	t.Cleanup(func() { data.Assets = savedAssets })

	destDir := filepath.Join(t.TempDir(), "artifacts")
	extracted, err := ExtractProvider(AWS.Name, destDir)
	require.NoError(t, err)

	// Both halves are needed. The binary alone cannot start, because the
	// provider's CRDs have to reach the local control plane first.
	binary, err := os.Stat(extracted.BinaryPath)
	require.NoError(t, err)
	assert.NotZero(t, binary.Size())
	assert.NotZero(t, binary.Mode()&0o111, "the extracted binary must be executable")

	components, err := os.Stat(extracted.ComponentsPath)
	require.NoError(t, err)
	assert.NotZero(t, components.Size())

	// The digest is what ties an install back to an exact build, so an empty
	// or placeholder value would defeat the point of logging it.
	assert.Len(t, extracted.BinarySHA256, 64)

	// The extracted artifacts must satisfy the same validation the
	// install-config paths go through, or the command produces something the
	// installer will then reject.
	assert.NoError(t, validateExecutable(extracted.BinaryPath))
	assert.NoError(t, validateComponents(extracted.ComponentsPath))

	// dest-dir is created if absent: a user should not have to mkdir first.
	assert.DirExists(t, destDir)
}
