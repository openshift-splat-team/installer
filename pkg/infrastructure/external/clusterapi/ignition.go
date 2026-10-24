package clusterapi

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	infracapi "github.com/openshift/installer/pkg/infrastructure/clusterapi"
)

// Ignition publishes the ignition configs as Cluster API bootstrap Secrets,
// one per machine role.
//
// The shared path already does this for providers that do not implement
// IgnitionProvider, but it creates only two Secrets -- `<infraID>-bootstrap`
// and `<infraID>-master` (pkg/infrastructure/clusterapi/clusterapi.go:367-369).
// That is the right default for an integrated platform, where workers are not
// created by Cluster API at all: they arrive after bootstrap, from machine-api
// MachineSets reconciled by an in-cluster actuator for that cloud.
//
// A non-integrated provider ships no machine-api actuator, so on this platform
// nothing ever creates a worker. Without a third Secret the cluster is capped
// at its control plane forever, because a worker Machine's
// spec.bootstrap.dataSecretName would have nothing valid to point at. Pointing
// it at `<infraID>-master` is not a workaround -- it would boot the node with
// the master pointer config and it would join as a control-plane node.
//
// So External implements this interface purely to add `<infraID>-worker`.
// Doing it here rather than in the shared `else` branch is deliberate: that
// branch is reached by every platform that does not override it, and adding a
// Secret there would change what those platforms create in the local control
// plane. This costs one small method and leaves them untouched.
//
// WorkerIgnData is the pointer config -- it fetches the real config from the
// machine config server at api-int:22623/config/worker -- so the Secret is
// small, and it is served by the bootstrap node's MCS until the permanent one
// takes over. A worker created before bootstrap completes therefore retries
// until the MCS answers, rather than failing.
//
// The Secret is created whether or not any worker Machine references it. It is
// inert if unused, it lives only in the installer's temporary local control
// plane, and creating it unconditionally means the user can add a worker
// manifest without also having to know that a flag elsewhere controls whether
// its bootstrap data exists.
func (p Provider) Ignition(_ context.Context, in infracapi.IgnitionInput) ([]*corev1.Secret, error) {
	secrets := []*corev1.Secret{
		infracapi.IgnitionSecret(in.BootstrapIgnData, in.InfraID, "bootstrap"),
		infracapi.IgnitionSecret(in.MasterIgnData, in.InfraID, "master"),
	}

	// Defensive: the worker ignition asset is generated for every platform
	// (pkg/asset/targets/targets.go:62,71), so this is expected to be set.
	// An empty Secret would be worse than an absent one, because a Machine
	// referencing it would provision and then fail to boot with nothing
	// pointing at the cause.
	if len(in.WorkerIgnData) > 0 {
		secrets = append(secrets, infracapi.IgnitionSecret(in.WorkerIgnData, in.InfraID, "worker"))
	}

	return secrets, nil
}
