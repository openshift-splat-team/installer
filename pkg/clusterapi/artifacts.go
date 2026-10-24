package clusterapi

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	// artifactEnvPrefix namespaces the developer-only Cluster API artifact
	// overrides. These are not a supported installation interface: they are
	// absent from install-config, unversioned, and may be removed at any time.
	artifactEnvPrefix = "OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_"

	binaryEnvSuffix     = "_BINARY"
	componentsEnvSuffix = "_COMPONENTS"
)

// artifactOverride describes developer-supplied Cluster API provider artifacts
// that replace the copies shipped inside the installer binary.
type artifactOverride struct {
	// BinaryPath is the validated, absolute path to the provider controller
	// executable.
	BinaryPath string
	// ComponentsPath is the validated, absolute path to the provider component
	// manifest, or to a directory of manifests.
	ComponentsPath string
	// BinarySHA256 is the hex-encoded digest of BinaryPath, recorded so that a
	// pilot run can be traced back to an exact build.
	BinarySHA256 string
}

// artifactEnvNames returns the environment variable names carrying the binary
// and component manifest overrides for the named provider.
func artifactEnvNames(provider string) (binary, components string) {
	name := strings.ToUpper(strings.ReplaceAll(provider, "-", "_"))
	return artifactEnvPrefix + name + binaryEnvSuffix,
		artifactEnvPrefix + name + componentsEnvSuffix
}

// lookupArtifactOverride reads the developer-only overrides for the named
// provider. It returns a nil override when none is configured, which is the
// default and the only supported configuration.
func lookupArtifactOverride(provider string) (*artifactOverride, error) {
	binaryEnv, componentsEnv := artifactEnvNames(provider)
	binaryPath := os.Getenv(binaryEnv)
	componentsPath := os.Getenv(componentsEnv)
	if binaryPath == "" && componentsPath == "" {
		return nil, nil
	}

	override := &artifactOverride{}
	if binaryPath != "" {
		// Resolve before validating. ct.Path is handed to process.State.Path
		// (system.go:723) and from there to exec.CommandContext
		// (internal/process/process.go:130), which resolves a name containing
		// no separator through $PATH. Validating the relative form would mean
		// stat'ing and hashing one file while running another.
		abs, err := absolutePath(binaryPath)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", binaryEnv, err)
		}
		if err := validateExecutable(abs); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", binaryEnv, err)
		}
		digest, err := fileSHA256(abs)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", binaryEnv, err)
		}
		override.BinaryPath, override.BinarySHA256 = abs, digest
	}
	if componentsPath != "" {
		abs, err := absolutePath(componentsPath)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", componentsEnv, err)
		}
		if err := validateComponents(abs); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", componentsEnv, err)
		}
		override.ComponentsPath = abs
	}
	return override, nil
}

// absolutePath returns the cleaned, absolute form of path, so that everything
// downstream -- the stat, the digest and the exec -- refers to the same file
// regardless of the installer's working directory.
func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q to an absolute path: %w", path, err)
	}
	return filepath.Clean(abs), nil
}

// validateRegularFile ensures path names an existing, non-empty regular file,
// and returns the stat result so that callers needing the mode do not have to
// stat twice. Empty files are rejected here: a zero-length download is a
// plausible failure mode and parses as neither a binary nor a manifest.
func validateRegularFile(path string) (fs.FileInfo, error) {
	// #nosec G703 -- developer-only override path: it comes from an opt-in
	// environment variable read in lookupArtifactOverride, is made absolute by
	// absolutePath before it gets here, and this function is the validation.
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory, want a regular file", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("%q is empty", path)
	}
	return info, nil
}

// validateComponents ensures path names something envtest can install from:
// either a manifest file or a directory of manifests. Both
// envtest.CRDInstallOptions and envtest.WebhookInstallOptions branch on
// info.IsDir() and read the directory
// (vendor/sigs.k8s.io/controller-runtime/pkg/envtest/crd.go:303-313,
// webhook.go:384-390), so a kustomize output directory is a legitimate
// override. The existence check stays because CRDInstallOptions
// .ErrorIfPathMissing defaults to false (crd.go:296-301): without it a
// mistyped path would be silently ignored rather than reported.
func validateComponents(path string) error {
	// #nosec G703 -- developer-only override path, made absolute by
	// absolutePath, and this function is the validation.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if info.IsDir() {
		return nil
	}
	if _, err := validateRegularFile(path); err != nil {
		return err
	}
	return nil
}

// validateExecutable ensures path names a regular, executable file built for
// the architecture this installer is running on. The check runs when the
// controller is started, which is after the provider's PreProvision hook and
// after the local control plane is up
// (pkg/infrastructure/clusterapi/clusterapi.go:138 and :172): it is early, but
// it is not ahead of every cloud call -- AWS creates IAM roles in PreProvision
// (pkg/infrastructure/aws/clusterapi/aws.go:58-61).
func validateExecutable(path string) error {
	info, err := validateRegularFile(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%q is not executable", path)
	}
	return validateHostArch(path)
}

// elfTarget is the ELF identity a provider binary must have to run on a given
// GOARCH. The machine type alone is not sufficient: EM_PPC64 is shared by
// big-endian ppc64 and little-endian ppc64le, and EM_S390 by 31-bit s390 and
// 64-bit s390x, so the class and the data encoding are compared as well.
type elfTarget struct {
	machine elf.Machine
	class   elf.Class
	data    elf.Data
}

// elfTargetForGOARCH maps the architectures the installer is released for to
// the ELF identity expected of a provider binary running on them.
var elfTargetForGOARCH = map[string]elfTarget{
	"amd64":   {machine: elf.EM_X86_64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	"arm64":   {machine: elf.EM_AARCH64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	"ppc64le": {machine: elf.EM_PPC64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	"s390x":   {machine: elf.EM_S390, class: elf.ELFCLASS64, data: elf.ELFDATA2MSB},
}

// elfMagic is the four-byte prefix every ELF file begins with.
var elfMagic = []byte{0x7f, 'E', 'L', 'F'}

// validateHostArch compares the ELF identity of path against the running
// installer's architecture. Files that are not ELF at all -- Mach-O on darwin,
// or a #!/bin/sh wrapper -- are accepted without a check rather than rejected,
// so that the override stays usable on developer workstations. A file that
// claims to be ELF but cannot be parsed is a different matter: that is a
// damaged artifact, and it is rejected.
func validateHostArch(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		if looksLikeELF(path) {
			return fmt.Errorf("%q looks like an ELF binary but could not be parsed: %w", path, err)
		}
		// The parse error is carried into the message: an ELF that merely
		// could not be read reports why, rather than being mislabelled as a
		// foreign executable format.
		logrus.Debugf("Skipping architecture check for %s: not a readable ELF binary: %v", path, err)
		return nil
	}
	defer f.Close()

	want, ok := elfTargetForGOARCH[runtime.GOARCH]
	if !ok {
		logrus.Debugf("Skipping architecture check for %s: no ELF target known for %s",
			path, runtime.GOARCH)
		return nil
	}
	if f.Machine != want.machine || f.Class != want.class || f.Data != want.data {
		return fmt.Errorf("%q is built for %s (%s, %s), but this installer is %s",
			path, f.Machine, f.Class, f.Data, runtime.GOARCH)
	}
	return nil
}

// looksLikeELF reports whether path begins with the ELF magic number. A file
// that cannot be opened or is shorter than the magic is reported as false: the
// caller is already handling an elf.Open failure, and the harder error is only
// warranted when the file is unambiguously meant to be an ELF.
func looksLikeELF(path string) bool {
	// #nosec G703 -- developer-only override path, validated by
	// validateRegularFile before it is read.
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	magic := make([]byte, len(elfMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		return false
	}
	return bytes.Equal(magic, elfMagic)
}

// fileSHA256 returns the hex-encoded SHA-256 digest of path. The digest is
// logged with the artifact source; the file contents never are.
func fileSHA256(path string) (string, error) {
	// #nosec G703 -- developer-only override path, validated by
	// validateRegularFile before it is hashed.
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open %q: %w", path, err)
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("cannot hash %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// applyArtifactOverride points the controller at developer-supplied artifacts
// and records their provenance. Only the fields the override actually sets are
// replaced, so a binary-only override keeps the embedded component manifest.
// The caller is responsible for ensuring ct.Provider is not nil.
func applyArtifactOverride(ct *controller, override *artifactOverride) {
	if override.BinaryPath != "" {
		logrus.Warnf("Using developer-supplied %s controller binary %s (sha256:%s); "+
			"this is not a supported configuration",
			ct.Provider.Name, override.BinaryPath, override.BinarySHA256)
		ct.Path = override.BinaryPath
		ct.skipExtract = true
	}
	if override.ComponentsPath != "" {
		logrus.Warnf("Using developer-supplied %s component manifest %s; "+
			"this is not a supported configuration",
			ct.Provider.Name, override.ComponentsPath)
		ct.Components = []string{override.ComponentsPath}
	}
}
