package clusterapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infracapi "github.com/openshift/installer/pkg/infrastructure/clusterapi"
)

// TestIgnitionPublishesAWorkerSecret pins the one thing this implementation
// exists for. The shared path creates bootstrap and master only, and on a
// platform with no machine-api actuator that caps the cluster at its control
// plane, because a worker Machine has no bootstrap Secret to reference.
func TestIgnitionPublishesAWorkerSecret(t *testing.T) {
	names := func(t *testing.T, in infracapi.IgnitionInput) []string {
		t.Helper()
		secrets, err := Provider{}.Ignition(context.Background(), in)
		require.NoError(t, err)
		got := make([]string, 0, len(secrets))
		for _, s := range secrets {
			got = append(got, s.Name)
		}
		return got
	}

	t.Run("all three roles, named for the infrastructure ID", func(t *testing.T) {
		got := names(t, infracapi.IgnitionInput{
			InfraID:          "mrb-ext12-q7gw7",
			BootstrapIgnData: []byte("bootstrap"),
			MasterIgnData:    []byte("master"),
			WorkerIgnData:    []byte("worker"),
		})
		assert.Equal(t, []string{
			"mrb-ext12-q7gw7-bootstrap",
			"mrb-ext12-q7gw7-master",
			"mrb-ext12-q7gw7-worker",
		}, got)
	})

	// An empty Secret is worse than an absent one: a Machine referencing it
	// provisions and then fails to boot with nothing naming the cause.
	t.Run("no worker Secret when there is no worker ignition", func(t *testing.T) {
		got := names(t, infracapi.IgnitionInput{
			InfraID:          "mrb-ext12-q7gw7",
			BootstrapIgnData: []byte("bootstrap"),
			MasterIgnData:    []byte("master"),
		})
		assert.Equal(t, []string{
			"mrb-ext12-q7gw7-bootstrap",
			"mrb-ext12-q7gw7-master",
		}, got)
	})

	t.Run("the worker Secret carries the worker ignition, in CAPI's format", func(t *testing.T) {
		secrets, err := Provider{}.Ignition(context.Background(), infracapi.IgnitionInput{
			InfraID:          "infra",
			BootstrapIgnData: []byte("b"),
			MasterIgnData:    []byte("m"),
			WorkerIgnData:    []byte("worker-pointer-config"),
		})
		require.NoError(t, err)
		require.Len(t, secrets, 3)
		worker := secrets[2]
		assert.Equal(t, []byte("worker-pointer-config"), worker.Data["value"])
		assert.Equal(t, []byte("ignition"), worker.Data["format"])
	})
}
