package clusterapi

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// ExternalProviderSpec describes a Cluster API infrastructure provider that the
// installer was not compiled against, supplied by the user for the External
// platform. Its paths are absolute and have been validated.
type ExternalProviderSpec struct {
	// Name identifies the provider. It names the controller in logs and
	// selects the developer-only artifact override environment variables.
	Name string
	// BinaryPath is the absolute path to the provider controller executable.
	BinaryPath string
	// ComponentsPath is the absolute path to the provider's CRDs and component
	// manifests, either a file or a directory.
	ComponentsPath string
	// BinarySHA256 is the hex-encoded digest of BinaryPath, recorded so that a
	// run can be traced back to an exact build.
	BinarySHA256 string
	// Args are appended to the controller command line after the arguments the
	// installer supplies itself.
	Args []string
}

// externalProvider carries the spec from the External infrastructure
// provider's PreProvision hook to System.Run.
//
// It is process state rather than an argument because System.Run reads
// metadata.json, not the install-config, and the two are reached through
// different call paths. PreProvision runs strictly before System.Run
// (pkg/infrastructure/clusterapi/clusterapi.go:138 and :172), so the spec is
// always set by the time it is read.
var externalProvider struct {
	sync.Mutex
	spec *ExternalProviderSpec
}

// SetExternalProvider validates the user-supplied provider artifacts and
// records them for System.Run.
//
// Validation is the same as for the developer-only environment-variable
// overrides -- the paths are made absolute, the binary must be a non-empty
// executable built for this host architecture, and the components must exist
// -- so that the two ways of supplying artifacts cannot disagree about what is
// acceptable. Only the artifact path, its source and its digest are logged;
// the contents never are.
func SetExternalProvider(spec *ExternalProviderSpec) error {
	if spec == nil {
		return fmt.Errorf("no Cluster API provider configured")
	}
	if spec.Name == "" {
		return fmt.Errorf("the Cluster API provider name is empty")
	}

	resolved := &ExternalProviderSpec{Name: spec.Name, Args: spec.Args}

	binary, err := absolutePath(spec.BinaryPath)
	if err != nil {
		return fmt.Errorf("invalid provider binary: %w", err)
	}
	if err := validateExecutable(binary); err != nil {
		return fmt.Errorf("invalid provider binary: %w", err)
	}
	digest, err := fileSHA256(binary)
	if err != nil {
		return fmt.Errorf("invalid provider binary: %w", err)
	}
	resolved.BinaryPath, resolved.BinarySHA256 = binary, digest

	components, err := absolutePath(spec.ComponentsPath)
	if err != nil {
		return fmt.Errorf("invalid provider components: %w", err)
	}
	if err := validateComponents(components); err != nil {
		return fmt.Errorf("invalid provider components: %w", err)
	}
	resolved.ComponentsPath = components

	// Warn, not Info, and say so plainly: this is a provisional interface and
	// a support case needs the signal in the log bundle. Matches the tone of
	// applyArtifactOverride for the developer-only env-var path.
	logrus.Warnf("Using user-supplied Cluster API infrastructure provider %q: binary %s (sha256:%s), components %s; "+
		"this is not a supported configuration",
		resolved.Name, resolved.BinaryPath, resolved.BinarySHA256, resolved.ComponentsPath)

	externalProvider.Lock()
	defer externalProvider.Unlock()
	externalProvider.spec = resolved
	return nil
}

// getExternalProvider returns the spec recorded by SetExternalProvider, or nil
// when none was configured.
func getExternalProvider() *ExternalProviderSpec {
	externalProvider.Lock()
	defer externalProvider.Unlock()
	return externalProvider.spec
}

// externalInfrastructureController builds the controller for a user-supplied
// infrastructure provider.
//
// Unlike the integrated providers it takes nothing from the embedded mirror:
// Path is the user's binary and Components is the user's manifest, so
// skipExtract is set and Provider.Sources is left empty. Provider itself stays
// non-nil so that the developer-only environment-variable overrides still
// apply -- an override for this provider name takes precedence over the
// install-config, which is what makes it useful for swapping a build without
// editing the install-config.
//
// The argument list is deliberately the smallest set every Cluster API
// provider is expected to accept. Whether that is genuinely true of providers
// other than the ones tested here is unproven; spec.Args exists so that a
// provider needing more can be configured without a code change. Note that
// runController also appends --kubeconfig, which is therefore required too.
func (c *system) externalInfrastructureController(spec *ExternalProviderSpec) *controller {
	args := []string{
		"-v=2",
		"--health-addr={{suggestHealthHostPort}}",
		"--webhook-port={{.WebhookPort}}",
		"--webhook-cert-dir={{.WebhookCertDir}}",
	}
	args = append(args, spec.Args...)

	return &controller{
		Provider:    &Provider{Name: spec.Name},
		Name:        fmt.Sprintf("%s infrastructure provider", spec.Name),
		Path:        spec.BinaryPath,
		Components:  []string{spec.ComponentsPath},
		Args:        args,
		Env:         map[string]string{},
		skipExtract: true,
	}
}
