package clusterapi

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: the tests in this file drive the overrides through the process
// environment with t.Setenv, and one of them changes directory with t.Chdir.
// Both panic in a test that has called t.Parallel, and both mutate
// process-wide state that a parallel test elsewhere in this file would race
// against. Nothing in this file may call t.Parallel.

func TestArtifactEnvNames(t *testing.T) {
	cases := []struct {
		provider       string
		wantBinary     string
		wantComponents string
	}{{
		provider:       "aws",
		wantBinary:     "OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_AWS_BINARY",
		wantComponents: "OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_AWS_COMPONENTS",
	}, {
		// This row exercises the hyphen-to-underscore name transform only.
		// The core Cluster API controller is not overridable: it is built in
		// system.go:161-174 with a nil Provider, and both the override lookup
		// and Extract are behind ct.Provider != nil, so setting
		// _CLUSTER_API_BINARY has no effect.
		provider:       "cluster-api",
		wantBinary:     "OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_CLUSTER_API_BINARY",
		wantComponents: "OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_CLUSTER_API_COMPONENTS",
	}}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			binary, components := artifactEnvNames(tc.provider)
			assert.Equal(t, tc.wantBinary, binary)
			assert.Equal(t, tc.wantComponents, components)
		})
	}
}

func TestLookupArtifactOverride(t *testing.T) {
	dir := t.TempDir()

	// A real ELF of the right architecture: the test binary itself.
	selfPath, err := os.Executable()
	assert.NoError(t, err)

	notExecutable := filepath.Join(dir, "not-executable")
	assert.NoError(t, os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o600))
	empty := filepath.Join(dir, "empty")
	assert.NoError(t, os.WriteFile(empty, nil, 0o600))
	components := filepath.Join(dir, "aws-infrastructure-components.yaml")
	assert.NoError(t, os.WriteFile(components, []byte("kind: List\n"), 0o600))

	// A kustomize build normally lands a directory of manifests, which envtest
	// installs just as happily as a single file.
	componentsDir := filepath.Join(dir, "components.d")
	assert.NoError(t, os.Mkdir(componentsDir, 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(componentsDir, "crd.yaml"), []byte("kind: List\n"), 0o600))

	cases := []struct {
		name       string
		binary     string
		components string
		wantNil    bool
		wantErr    string
	}{{
		name:    "no override is the default",
		wantNil: true,
	}, {
		name:   "binary only",
		binary: selfPath,
	}, {
		name:       "components only",
		components: components,
	}, {
		name:       "components may be a directory",
		components: componentsDir,
	}, {
		name:       "binary and components",
		binary:     selfPath,
		components: components,
	}, {
		name:    "missing path",
		binary:  filepath.Join(dir, "absent"),
		wantErr: "cannot stat",
	}, {
		name:    "directory",
		binary:  dir,
		wantErr: "is a directory",
	}, {
		name:    "not executable",
		binary:  notExecutable,
		wantErr: "is not executable",
	}, {
		name:    "empty binary",
		binary:  empty,
		wantErr: "is empty",
	}, {
		name:       "missing components",
		binary:     selfPath,
		components: filepath.Join(dir, "absent.yaml"),
		wantErr:    "cannot stat",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binaryEnv, componentsEnv := artifactEnvNames("aws")
			t.Setenv(binaryEnv, tc.binary)
			t.Setenv(componentsEnv, tc.components)

			got, err := lookupArtifactOverride("aws")
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			if !assert.NoError(t, err) {
				return
			}

			if tc.wantNil {
				assert.Nil(t, got)
				return
			}
			if !assert.NotNil(t, got) {
				return
			}
			assert.Equal(t, tc.binary, got.BinaryPath)
			assert.Equal(t, tc.components, got.ComponentsPath)
			if tc.binary != "" {
				assert.Len(t, got.BinarySHA256, 64, "want a hex-encoded SHA-256 digest")
			} else {
				assert.Empty(t, got.BinarySHA256, "no binary was overridden")
			}
		})
	}
}

// TestLookupArtifactOverrideResolvesRelativePaths pins the rule that the file
// validated and hashed is the file that will run. ct.Path becomes
// process.State.Path (system.go:723) and then the Path of an exec.Cmd
// (internal/process/process.go:130), which resolves a separator-free name
// through $PATH -- so a bare basename must be made absolute before it is
// stat'd, or the digest would describe a different file from the one executed.
func TestLookupArtifactOverrideResolvesRelativePaths(t *testing.T) {
	selfPath, err := os.Executable()
	assert.NoError(t, err)

	dir := t.TempDir()
	const (
		binaryBase     = "cluster-api-provider-aws"
		componentsBase = "aws-infrastructure-components.yaml"
	)
	// A symlink keeps the fixture executable without copying the test binary
	// or widening any file mode.
	assert.NoError(t, os.Symlink(selfPath, filepath.Join(dir, binaryBase)))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, componentsBase), []byte("kind: List\n"), 0o600))

	t.Chdir(dir)

	binaryEnv, componentsEnv := artifactEnvNames("aws")
	t.Setenv(binaryEnv, binaryBase)
	t.Setenv(componentsEnv, componentsBase)

	got, err := lookupArtifactOverride("aws")
	if !assert.NoError(t, err) || !assert.NotNil(t, got) {
		return
	}
	assert.True(t, filepath.IsAbs(got.BinaryPath),
		"BinaryPath = %q, want an absolute path", got.BinaryPath)
	assert.Equal(t, binaryBase, filepath.Base(got.BinaryPath))
	assert.True(t, filepath.IsAbs(got.ComponentsPath),
		"ComponentsPath = %q, want an absolute path", got.ComponentsPath)
	assert.Equal(t, componentsBase, filepath.Base(got.ComponentsPath))
}

// TestLookupArtifactOverrideIsPerProvider checks that an override for one
// provider does not leak into another: only the named provider's variables are
// read, so overriding AWS leaves every other controller on the embedded path.
func TestLookupArtifactOverrideIsPerProvider(t *testing.T) {
	selfPath, err := os.Executable()
	assert.NoError(t, err)

	awsBinaryEnv, _ := artifactEnvNames("aws")
	t.Setenv(awsBinaryEnv, selfPath)

	got, err := lookupArtifactOverride("gcp")
	assert.NoError(t, err)
	assert.Nil(t, got, "no override was configured for gcp")
}

// TestRunControllerDefaultsAreUntouched is the regression guarantee: with
// neither variable set, the lookup yields no override and the controller keeps
// the embedded binary, the embedded components, and extraction enabled. The
// body mirrors the guard in runController (system.go), which only calls
// applyArtifactOverride when the lookup returns non-nil.
func TestRunControllerDefaultsAreUntouched(t *testing.T) {
	const (
		embeddedPath       = "/bin-dir/cluster-api-provider-aws"
		embeddedComponents = "/component-dir/aws-infrastructure-components.yaml"
	)

	binaryEnv, componentsEnv := artifactEnvNames("aws")
	t.Setenv(binaryEnv, "")
	t.Setenv(componentsEnv, "")

	ct := &controller{
		Provider:   &AWS,
		Name:       "aws infrastructure provider",
		Path:       embeddedPath,
		Components: []string{embeddedComponents},
	}

	override, err := lookupArtifactOverride(ct.Provider.Name)
	assert.NoError(t, err)
	if !assert.Nil(t, override) {
		return
	}
	if override != nil {
		applyArtifactOverride(ct, override)
	}

	assert.Equal(t, embeddedPath, ct.Path)
	assert.Equal(t, []string{embeddedComponents}, ct.Components)
	assert.False(t, ct.skipExtract, "the embedded binary must still be extracted")
}

// TestApplyArtifactOverride covers the three shapes an override can take, and
// asserts that a field the override does not set is left untouched.
func TestApplyArtifactOverride(t *testing.T) {
	const (
		embeddedPath       = "/bin-dir/cluster-api-provider-aws"
		embeddedComponents = "/component-dir/aws-infrastructure-components.yaml"
	)

	cases := []struct {
		name           string
		override       artifactOverride
		wantPath       string
		wantComponents []string
		wantSkip       bool
	}{{
		name:           "binary only keeps the embedded components",
		override:       artifactOverride{BinaryPath: "/tmp/capa", BinarySHA256: "abc"},
		wantPath:       "/tmp/capa",
		wantComponents: []string{embeddedComponents},
		wantSkip:       true,
	}, {
		name:           "components only keeps the embedded binary and still extracts",
		override:       artifactOverride{ComponentsPath: "/tmp/components.yaml"},
		wantPath:       embeddedPath,
		wantComponents: []string{"/tmp/components.yaml"},
		wantSkip:       false,
	}, {
		name: "both",
		override: artifactOverride{
			BinaryPath:     "/tmp/capa",
			BinarySHA256:   "abc",
			ComponentsPath: "/tmp/components.yaml",
		},
		wantPath:       "/tmp/capa",
		wantComponents: []string{"/tmp/components.yaml"},
		wantSkip:       true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controller{
				Provider:   &AWS,
				Name:       "aws infrastructure provider",
				Path:       embeddedPath,
				Components: []string{embeddedComponents},
			}

			applyArtifactOverride(ct, &tc.override)

			assert.Equal(t, tc.wantPath, ct.Path)
			assert.Equal(t, tc.wantSkip, ct.skipExtract)
			assert.Equal(t, tc.wantComponents, ct.Components)
			// The override must never disturb the provider, which
			// runController reads again for the azureaso special case.
			assert.NotNil(t, ct.Provider, "runController dereferences Provider later")
		})
	}
}

// writeELFHeader writes a minimal, well-formed ELF header -- no program
// headers and no sections -- and returns its path. debug/elf parses this
// happily, so every architecture the installer ships can be covered without
// checking a foreign binary into the tree.
func writeELFHeader(t *testing.T, dir, name string, target elfTarget) string {
	t.Helper()

	var order binary.ByteOrder = binary.LittleEndian
	if target.data == elf.ELFDATA2MSB {
		order = binary.BigEndian
	}

	ident := make([]byte, 16)
	copy(ident, elfMagic)
	ident[elf.EI_CLASS] = byte(target.class)
	ident[elf.EI_DATA] = byte(target.data)
	ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)

	buf := &bytes.Buffer{}
	_, err := buf.Write(ident)
	assert.NoError(t, err)
	put16 := func(v uint16) { assert.NoError(t, binary.Write(buf, order, v)) }
	put32 := func(v uint32) { assert.NoError(t, binary.Write(buf, order, v)) }
	put64 := func(v uint64) { assert.NoError(t, binary.Write(buf, order, v)) }

	put16(uint16(elf.ET_EXEC))
	put16(uint16(target.machine))
	put32(uint32(elf.EV_CURRENT))
	if target.class == elf.ELFCLASS32 {
		put32(0)  // e_entry
		put32(0)  // e_phoff
		put32(0)  // e_shoff
		put32(0)  // e_flags
		put16(52) // e_ehsize
	} else {
		put64(0)  // e_entry
		put64(0)  // e_phoff
		put64(0)  // e_shoff
		put32(0)  // e_flags
		put16(64) // e_ehsize
	}
	put16(0) // e_phentsize
	put16(0) // e_phnum
	put16(0) // e_shentsize
	put16(0) // e_shnum
	put16(0) // e_shstrndx

	path := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

// TestValidateHostArch drives the architecture check over every architecture
// the installer is released for. The expected ELF triples are spelled out here
// rather than read from elfTargetForGOARCH, so that a typo in a row for an
// architecture this test is not running on is still caught.
func TestValidateHostArch(t *testing.T) {
	cases := []struct {
		goarch string
		target elfTarget
	}{{
		goarch: "amd64",
		target: elfTarget{machine: elf.EM_X86_64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	}, {
		goarch: "arm64",
		target: elfTarget{machine: elf.EM_AARCH64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	}, {
		// EM_PPC64 is shared with big-endian ppc64, so the data encoding is
		// what distinguishes them.
		goarch: "ppc64le",
		target: elfTarget{machine: elf.EM_PPC64, class: elf.ELFCLASS64, data: elf.ELFDATA2LSB},
	}, {
		// EM_S390 is shared with 31-bit s390, so the class is what
		// distinguishes them.
		goarch: "s390x",
		target: elfTarget{machine: elf.EM_S390, class: elf.ELFCLASS64, data: elf.ELFDATA2MSB},
	}}

	assert.Len(t, elfTargetForGOARCH, len(cases), "every mapped GOARCH must be covered here")

	dir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.goarch, func(t *testing.T) {
			assert.Equal(t, tc.target, elfTargetForGOARCH[tc.goarch])

			path := writeELFHeader(t, dir, tc.goarch, tc.target)
			err := validateHostArch(path)
			if tc.goarch == runtime.GOARCH {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, "is built for")
			}
		})
	}

	host, ok := elfTargetForGOARCH[runtime.GOARCH]
	if !ok {
		t.Skipf("no ELF target known for %s; the mismatch cases need a host entry", runtime.GOARCH)
	}

	// A real ELF of the right architecture: the test binary itself.
	selfPath, err := os.Executable()
	assert.NoError(t, err)
	assert.NoError(t, validateHostArch(selfPath))

	otherMachine := elf.EM_X86_64
	if host.machine == otherMachine {
		otherMachine = elf.EM_AARCH64
	}
	otherData := elf.ELFDATA2MSB
	if host.data == otherData {
		otherData = elf.ELFDATA2LSB
	}

	mismatches := []struct {
		name   string
		target elfTarget
	}{{
		name:   "wrong machine",
		target: elfTarget{machine: otherMachine, class: host.class, data: host.data},
	}, {
		name:   "wrong class",
		target: elfTarget{machine: host.machine, class: elf.ELFCLASS32, data: host.data},
	}, {
		name:   "wrong endianness",
		target: elfTarget{machine: host.machine, class: host.class, data: otherData},
	}}

	for _, tc := range mismatches {
		t.Run(tc.name, func(t *testing.T) {
			path := writeELFHeader(t, dir, tc.name, tc.target)
			assert.ErrorContains(t, validateHostArch(path), "is built for")
		})
	}
}

// TestValidateHostArchDamagedELF covers the difference between a file that is
// not an ELF -- acceptable, because the override has to stay usable where the
// native format is something else -- and a file that claims to be an ELF but
// cannot be parsed, which is a damaged artifact and would die at exec with
// "exec format error".
func TestValidateHostArchDamagedELF(t *testing.T) {
	dir := t.TempDir()

	truncated := filepath.Join(dir, "truncated")
	assert.NoError(t, os.WriteFile(truncated, []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01}, 0o600))
	assert.ErrorContains(t, validateHostArch(truncated),
		"looks like an ELF binary but could not be parsed")

	// A zero-length file never reaches the architecture check: an interrupted
	// download is rejected as empty by validateRegularFile first.
	empty := filepath.Join(dir, "empty")
	assert.NoError(t, os.WriteFile(empty, nil, 0o600))
	assert.ErrorContains(t, validateExecutable(empty), "is empty")

	// Deliberately still accepted: a wrapper script, or any other executable
	// that is simply not in ELF format.
	script := filepath.Join(dir, "wrapper.sh")
	assert.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o600))
	assert.NoError(t, validateHostArch(script))
}

func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	assert.NoError(t, os.WriteFile(path, nil, 0o600))

	// The SHA-256 of the empty string.
	const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got, err := fileSHA256(path)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = fileSHA256(filepath.Join(t.TempDir(), "absent"))
	assert.Error(t, err, "a missing file must not hash")
}
