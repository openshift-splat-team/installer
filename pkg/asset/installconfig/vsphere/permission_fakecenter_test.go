package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	fakecenter "github.com/openshift-splat-team/fake-center/helper"
	"github.com/openshift/installer/pkg/types/vsphere"
)

// skipWithoutFakeCenter skips scenario-backed tests unless a fake-center image
// is pinned and a container runtime is available.
func skipWithoutFakeCenter(t *testing.T) {
	t.Helper()
	if os.Getenv("FAKE_CENTER_IMAGE") == "" {
		t.Skip("set FAKE_CENTER_IMAGE to run fake-center scenario tests")
	}
	for _, runtime := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(runtime); err == nil {
			return
		}
	}
	t.Skip("podman or docker required for fake-center scenario tests")
}

// connectVCenter returns a vim25 client logged in as the given scenario
// account, trusting the instance CA.
func connectVCenter(t *testing.T, vcenter *fakecenter.VCenter, account fakecenter.AccountRef) (*vim25.Client, error) {
	t.Helper()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, vcenter.CABundle, 0600); err != nil {
		return nil, err
	}
	u, err := url.Parse(vcenter.Server)
	if err != nil {
		return nil, err
	}
	u.Path = "/sdk"
	soapClient := soap.NewClient(u, false)
	t.Cleanup(soapClient.CloseIdleConnections)
	if err := soapClient.SetRootCAs(caPath); err != nil {
		return nil, err
	}
	ctx := context.Background()
	client, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}
	sessionMgr := session.NewManager(client)
	if err := sessionMgr.Login(ctx, url.UserPassword(account.Username, account.Password)); err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_ = sessionMgr.Logout(ctx)
	})
	return client, nil
}

// scenarioTopology generates the failure-domain topology pointing at the
// inventory fake-center stages for the named failure domain.
func scenarioTopology(fd string) vsphere.Topology {
	return vsphere.Topology{
		Datacenter:     fd,
		ComputeCluster: fmt.Sprintf("/%s/host/%s-cluster", fd, fd),
		ResourcePool:   fmt.Sprintf("/%s/host/%s-cluster/Resources", fd, fd),
		Folder:         fmt.Sprintf("/%s/vm/%s-vms", fd, fd),
		Networks:       []string{fmt.Sprintf("%s-network", fd)},
		Datastore:      fmt.Sprintf("/%s/datastore/%s-datastore", fd, fd),
	}
}

// TestPermissionValidateScenarios exercises the privilege-validation code
// against a scenario-backed fake vCenter: each failure domain stages a
// deliberate privilege gap, and the real SOAP privilege fetch reports it.
func TestPermissionValidateScenarios(t *testing.T) {
	skipWithoutFakeCenter(t)

	vcenter, err := fakecenter.Start(t, "testdata/privileges.yaml")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		account   string
		fd        string
		modify    func(*vsphere.Topology)
		expectErr string
	}{
		{
			name:    "valid permissions",
			account: "installer",
			fd:      "fd-ok",
		},
		{
			name:      "missing portgroup permissions",
			account:   "installer",
			fd:        "fd-portgroup",
			expectErr: `privileges missing for vSphere Port Group: Network.Assign`,
		},
		{
			name:      "missing vCenter permissions",
			account:   "no-storageprofile",
			fd:        "fd-ok",
			expectErr: `privileges missing for vSphere vCenter: StorageProfile.View`,
		},
		{
			name:    "missing cluster permissions without a resource pool",
			account: "installer",
			fd:      "fd-cluster",
			modify: func(topology *vsphere.Topology) {
				topology.ResourcePool = ""
			},
			expectErr: `privileges missing for vSphere vCenter Cluster: VApp.AssignResourcePool`,
		},
		{
			name:    "read-only cluster tolerated with a resource pool",
			account: "installer",
			fd:      "fd-no-cluster",
		},
		{
			name:    "missing datacenter permissions without a folder",
			account: "installer",
			fd:      "fd-datacenter",
			modify: func(topology *vsphere.Topology) {
				topology.Folder = ""
			},
			expectErr: `privileges missing for vSphere vCenter Datacenter: Folder.Delete`,
		},
		{
			name:    "read-only datacenter tolerated with a folder",
			account: "installer",
			fd:      "fd-no-datacenter",
		},
		{
			name:      "missing datastore permissions",
			account:   "installer",
			fd:        "fd-datastore",
			expectErr: `privileges missing for vSphere vCenter Datastore: Datastore.AllocateSpace`,
		},
		{
			name:      "missing resource pool permissions",
			account:   "installer",
			fd:        "fd-resourcepool",
			expectErr: `privileges missing for vSphere vCenter Resource Pool: VApp.AssignResourcePool`,
		},
		{
			name:    "missing folder permissions tolerated without a folder",
			account: "installer",
			fd:      "fd-no-folder",
			modify: func(topology *vsphere.Topology) {
				topology.Folder = ""
			},
		},
		{
			name:      "missing folder permissions",
			account:   "installer",
			fd:        "fd-no-folder",
			expectErr: `privileges missing for Pre-existing Virtual Machine Folder:`,
		},
		{
			// The privilege spans the datacenter, datastore, and folder
			// groups, so the aggregate also contains the datastore error;
			// the regexp targets the datacenter one.
			name:    "datacenter missing ObjectAttachable",
			account: "installer",
			fd:      "fd-objectattachable",
			modify: func(topology *vsphere.Topology) {
				topology.Folder = ""
			},
			expectErr: `privileges missing for vSphere vCenter Datacenter: [^,]*InventoryService.Tagging.ObjectAttachable`,
		},
		{
			name:    "invalid datacenter",
			account: "installer",
			fd:      "fd-ok",
			modify: func(topology *vsphere.Topology) {
				topology.Datacenter = "invalid"
			},
			expectErr: `datacenter 'invalid' not found`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, ok := vcenter.Account(test.account)
			if !ok {
				t.Fatalf("account %q missing from scenario", test.account)
			}
			client, err := connectVCenter(t, vcenter, account)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			validationCtx := &validationContext{
				AuthManager: object.NewAuthorizationManager(client),
				Finder:      find.NewFinder(client),
				Client:      client,
			}
			if err := pruneToAvailablePermissions(ctx, validationCtx.AuthManager); err != nil {
				t.Fatal(err)
			}
			for group, definition := range permissions {
				if len(definition.Permissions) == 0 {
					t.Fatalf("privilege group %q pruned to empty against the fake-center catalog", group)
				}
			}

			failureDomain := vsphere.FailureDomain{Name: test.fd, Topology: scenarioTopology(test.fd)}
			if test.modify != nil {
				test.modify(&failureDomain.Topology)
			}
			err = validateFailureDomain(validationCtx, &failureDomain, false).ToAggregate()
			if test.expectErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, test.expectErr, err)
			}
		})
	}
}
