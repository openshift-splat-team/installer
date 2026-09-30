// Package hooks runs the user-supplied programs that automate the parts of an
// External install the installer cannot do itself.
//
// The installer knows nothing about a partner's cloud, which is the whole
// premise of provisioning with a user-supplied Cluster API provider. But some
// of what an install needs is not expressible in the Cluster API contract at
// all. DNS is the case that forces this package to exist: a control-plane
// machine fetches its ignition from
// https://api-int.<clusterDomain>:22623/config/master and the bootstrap node's
// kubeconfig targets api-int as well, yet the core Cluster carries one
// spec.controlPlaneEndpoint and has no field for an internal endpoint. An
// integrated platform creates those records in its own InfraReady; this
// platform has to call out to the user instead.
//
// So a hook is the External platform's substitute for provider-specific
// installer code. It runs at the same point in the flow, is handed everything
// the installer can describe without knowing the provider's API, and reports
// failure the same way.
//
// This package is deliberately separate from the provider implementation so
// that `destroy cluster`, which has no install-config and no provider object,
// can run the teardown counterpart through exactly the same code.
package hooks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	externaltypes "github.com/openshift/installer/pkg/types/external"
)

// Kind names a point in the install at which a hook runs. The value is passed
// to the program, so one script can serve every hook and dispatch on it --
// which is what the reference implementation does, and what keeps a hook's
// create and delete halves in one file where they can be read against each
// other.
type Kind string

const (
	// InfraReady runs once the Cluster reports its infrastructure ready and
	// before any machine is created.
	InfraReady Kind = "infra-ready"

	// PreDestroy runs during `destroy cluster`, before the Cluster is deleted.
	PreDestroy Kind = "pre-destroy"
)

// Request is everything a hook is given.
//
// The fields divide in two. Most describe the cluster in terms the installer
// owns and every platform shares. ClusterJSON and InfraJSON are the exception
// and are the reason a hook can be useful at all: they are the Cluster API
// objects copied verbatim out of the installer's local control plane, so a
// hook can read provider-specific status -- a load balancer's DNS name, a
// network ID -- that the installer has no type for and must never grow one
// for.
type Request struct {
	// Kind is the hook being run.
	Kind Kind

	// Program is the path to the executable, relative to the External
	// manifest directory inside InstallDir.
	Program string

	// InstallDir is the install directory. It is the hook's working
	// directory and the root its Program is resolved against.
	InstallDir string

	// InfraID is the cluster's infrastructure ID, the name the provider's
	// objects and cloud resources carry.
	InfraID string

	// ClusterName and BaseDomain give the cluster's identity; their join is
	// the cluster domain, which is what DNS records are built from.
	ClusterName string
	BaseDomain  string

	// Publish is the install-config publishing strategy, "External" or
	// "Internal". A hook creating DNS needs it to decide whether a public
	// record is wanted at all.
	Publish string

	// ControlPlaneEndpointHost and ControlPlaneEndpointPort are
	// Cluster.spec.controlPlaneEndpoint. This is the only endpoint in the
	// core Cluster API contract, so it is the one address a hook can rely on
	// without reading provider-specific status.
	ControlPlaneEndpointHost string
	ControlPlaneEndpointPort string

	// ClusterJSON is the core Cluster object as JSON. Empty if unavailable.
	ClusterJSON []byte

	// InfraJSON is the provider's infrastructure object as JSON. Empty when
	// there is not exactly one, since a hook cannot be told which of several
	// was meant.
	InfraJSON []byte
}

// clusterDomain is the name the cluster's DNS records hang off.
func (r Request) clusterDomain() string {
	return fmt.Sprintf("%s.%s", r.ClusterName, r.BaseDomain)
}

// Run executes the hook and waits for it to finish.
//
// A non-zero exit fails the install or the destroy. That is the strict
// choice, and it is the right one: a hook exists because something the
// cluster needs is not being created by anything else, so a hook that failed
// means that thing does not exist. The alternative -- warn and continue -- is
// what produces the failure this whole mechanism was built to avoid, an
// install that runs for forty minutes and then dies at a bootstrap stage with
// no line pointing back at the cause.
func Run(ctx context.Context, req Request) error {
	program, err := resolveProgram(req.InstallDir, req.Program)
	if err != nil {
		return err
	}

	stateDir := filepath.Join(req.InstallDir, externaltypes.ManifestDir, externaltypes.HookStateDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("failed to create the hook state directory %s: %w", stateDir, err)
	}

	// The objects go in a directory that is removed when the hook returns.
	// They are a snapshot for the duration of one call, and leaving them in
	// the install directory would put a provider's whole object -- whose
	// contents the installer does not understand and cannot vouch for -- into
	// something that gets archived and shared.
	objDir, err := os.MkdirTemp("", "openshift-install-hook-")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory for the hook input: %w", err)
	}
	defer os.RemoveAll(objDir)

	env := os.Environ()
	env = append(env,
		"OPENSHIFT_INSTALL_HOOK="+string(req.Kind),
		"OPENSHIFT_INSTALL_INFRA_ID="+req.InfraID,
		"OPENSHIFT_INSTALL_CLUSTER_NAME="+req.ClusterName,
		"OPENSHIFT_INSTALL_BASE_DOMAIN="+req.BaseDomain,
		"OPENSHIFT_INSTALL_CLUSTER_DOMAIN="+req.clusterDomain(),
		"OPENSHIFT_INSTALL_PUBLISH="+req.Publish,
		"OPENSHIFT_INSTALL_DIR="+req.InstallDir,
		"OPENSHIFT_INSTALL_MANIFEST_DIR="+filepath.Join(req.InstallDir, externaltypes.ManifestDir),
		"OPENSHIFT_INSTALL_STATE_DIR="+stateDir,
		"OPENSHIFT_INSTALL_CONTROL_PLANE_ENDPOINT_HOST="+req.ControlPlaneEndpointHost,
		"OPENSHIFT_INSTALL_CONTROL_PLANE_ENDPOINT_PORT="+req.ControlPlaneEndpointPort,
	)

	for _, obj := range []struct {
		envVar string
		file   string
		data   []byte
	}{
		{"OPENSHIFT_INSTALL_CLUSTER_JSON", "cluster.json", req.ClusterJSON},
		{"OPENSHIFT_INSTALL_INFRA_JSON", "infrastructure.json", req.InfraJSON},
	} {
		if len(obj.data) == 0 {
			continue
		}
		path := filepath.Join(objDir, obj.file)
		if err := os.WriteFile(path, obj.data, 0o600); err != nil {
			return fmt.Errorf("failed to write the hook input %s: %w", path, err)
		}
		env = append(env, obj.envVar+"="+path)
	}

	// Path and digest, never contents: the same rule the provider artifact
	// resolver follows. The digest is what lets a failed install and a failed
	// destroy be compared -- two runs against different versions of a hook is
	// otherwise invisible.
	digest, err := sha256File(program)
	if err != nil {
		return err
	}
	logrus.Infof("Running the %s hook %s (sha256:%s)", req.Kind, program, digest)

	stdout := &logWriter{prefix: string(req.Kind)}
	stderr := &logWriter{prefix: string(req.Kind)}

	cmd := exec.CommandContext(ctx, program)
	cmd.Dir = req.InstallDir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	stdout.Flush()
	stderr.Flush()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return fmt.Errorf("the %s hook %s exited %d; its output is above",
				req.Kind, req.Program, exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run the %s hook %s: %w", req.Kind, req.Program, runErr)
	}
	logrus.Debugf("The %s hook completed", req.Kind)
	return nil
}

// resolveProgram turns the configured relative path into an absolute one,
// refusing anything that leaves the install directory or is not executable.
//
// Both checks are here rather than at validation time because both need the
// filesystem, and because this is the last moment before the program runs --
// a hook that was executable when the install-config was validated and is not
// now should fail with the same message.
func resolveProgram(installDir, program string) (string, error) {
	if program == "" {
		return "", fmt.Errorf("no hook program configured")
	}

	local, err := filepath.Localize(program)
	if err != nil {
		return "", fmt.Errorf("hook %q must be a relative path inside the %q directory: %w",
			program, externaltypes.ManifestDir, err)
	}
	path := filepath.Join(installDir, externaltypes.ManifestDir, local)

	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return "", fmt.Errorf("hook %s does not exist: it is named in "+
			"platform.external.clusterAPI.hooks and is expected at that path, relative to the %q directory "+
			"of the install directory", path, externaltypes.ManifestDir)
	case err != nil:
		return "", fmt.Errorf("failed to read the hook %s: %w", path, err)
	case info.IsDir():
		return "", fmt.Errorf("hook %s is a directory", path)
	case info.Mode().Perm()&0o111 == 0:
		return "", fmt.Errorf("hook %s is not executable: run chmod +x on it. "+
			"The installer executes it directly rather than through a shell, so the execute bit and, for a "+
			"script, a #! line are both required", path)
	}
	return path, nil
}

// sha256File digests a file without retaining its contents.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// logWriter forwards a child process's output to the installer log one line
// at a time, so that a hook's progress is interleaved with the installer's
// own rather than arriving in a block when the process exits.
type logWriter struct {
	prefix string
	buf    bytes.Buffer
}

// Write never fails, and says so, because it is handed to exec.Cmd as the
// child's stdout and stderr: an error returned here would stop the installer
// collecting a hook's output partway through, for no gain. The two writes it
// makes are to a bytes.Buffer, which documents its error as always nil and
// panics rather than returning one.
func (w *logWriter) Write(p []byte) (int, error) {
	w.buf.Write(p) //nolint:errcheck // bytes.Buffer.Write never returns an error.
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Partial line: put it back and wait for the rest.
			w.buf.Reset()
			w.buf.WriteString(line) //nolint:errcheck // as above.
			break
		}
		w.emit(line)
	}
	return len(p), nil
}

// Flush emits whatever the process wrote without a trailing newline.
func (w *logWriter) Flush() {
	if w.buf.Len() > 0 {
		w.emit(w.buf.String())
		w.buf.Reset()
	}
}

func (w *logWriter) emit(line string) {
	if line = strings.TrimRight(line, "\r\n"); line != "" {
		logrus.Infof("%s: %s", w.prefix, line)
	}
}
