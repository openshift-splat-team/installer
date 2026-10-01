# extra-manifests — and the one thing that deliberately is not here

**Status: never run.** Written 2026-09-30 against the CAPOCI and oci-openshift
clones, with no OCI account available.

Files in this directory are copied by the installer into the openshift manifest
directory (`pkg/asset/manifests/openshift.go`, `externalExtraManifests`),
collected by bootkube
(`data/data/bootstrap/files/usr/local/bin/bootkube.sh.template:78`), and applied
by cluster-bootstrap to the bootstrap control plane before it waits for the
required pods. That is the only moment at which a CCM is useful: it runs while
every node is still `NotReady` and carries
`node.cloudprovider.kubernetes.io/uninitialized`.

## What is here

| File | Purpose |
| --- | --- |
| `99_external-00-kubelet-providerid-master.yaml` | sets `KUBELET_PROVIDERID=oci://<instance OCID>` before kubelet starts |
| `99_external-00-kubelet-providerid-worker.yaml` | same, for workers |

Both are adapted from Oracle's own
`oci-openshift/custom_manifests/butane/oci-kubelet-providerid-{master,worker}.bu`,
which is in production use for Agent-based OCI installs. The script is hardened
relative to Oracle's in two ways, both noted in the file: it retries the
metadata fetch and fails loudly on an empty OCID, and it creates the kubelet
drop-in directory if it does not exist.

## What is NOT here, and why — the OCI CCM and CSI config

**The aws-capa example ships the CCM config as a day-0 manifest. This one
cannot, and that difference is the most instructive thing in this example.**

The OCI cloud controller manager and CSI driver read their configuration from
two Secrets, defined in
`oci-openshift/custom_manifests/manifests/01-oci-driver-configs.yml`:

| Secret | Namespace | Key |
| --- | --- | --- |
| `oci-cloud-controller-manager` | `oci-cloud-controller-manager` | `cloud-provider.yaml` |
| `oci-volume-provisioner` | `oci-csi` | `config.yaml` |

Both contain the same four values:

| Field | Source | Known at `create manifests` time? |
| --- | --- | --- |
| `compartment` | user input | **yes** |
| `vcn` | the VCN CAPOCI creates | **no** |
| `loadBalancer.subnet1` | the service-lb subnet CAPOCI creates | **no** |
| `loadBalancer.securityLists.<subnet>` | that subnet's security list | **no** |

Three of the four do not exist until CAPI has built the network, which happens
after manifests are rendered. So there is no day-0 manifest that could carry
them, and writing one with placeholders would produce a CCM that starts,
fails to find its VCN and crash-loops.

**`../hooks/infra-hook.sh` writes both Secrets at `infraReady` instead.** That
hook runs after `Cluster.status.infrastructureReady` is true and before any
machine is created (`pkg/infrastructure/clusterapi/clusterapi.go:312`), and it
reads the OCIDs out of `$OPENSHIFT_INSTALL_INFRA_JSON`:

```
.spec.networkSpec.vcn.id
.spec.networkSpec.vcn.subnets[] | select(.role=="service-lb") | .id
.spec.networkSpec.vcn.subnets[] | select(.role=="service-lb") | .securityList.id
```

Those are in the **spec**, not the status, because CAPOCI writes the OCIDs of
what it creates back into the spec — `cloud/scope/vcn_reconciler.go:43,51` for
the VCN and `cloud/scope/subnet_reconciler.go` (`desiredSubnet.ID = subnet.Id`)
for subnets and their security lists. `OCIClusterStatus` has no network block at
all (`api/v1beta2/ocicluster_types.go:85-95`), so anyone looking for the
`AWSCluster.status.network` equivalent will not find one.

### There are no credentials in either Secret

Both set `useInstancePrincipals: true`. The in-cluster OCI components
authenticate as the instance, through a dynamic group and an IAM policy created
out of band. **No OCI API key reaches the installed cluster.** That is a real
advantage over the AWS example, which ships
`99_external-01-ccm-credentials.yaml`.

The OCI API key that CAPOCI itself uses is a different thing entirely: it lives
only in the installer's temporary local control plane and is never delivered
anywhere. See `../cluster.yaml`.

### The CCM deployment itself

Not shipped here either, and that is a scope decision rather than a technical
constraint. Oracle ships a complete, self-contained bundle —
`oci-openshift/custom_manifests/manifests/` has the CCM DaemonSet, the CSI
driver, RBAC and the CSI storage class. Those manifests are reusable as-is; they
need no value that is unknown at day 0, only the two config Secrets above.

Copy them in when the pilot gets far enough to need a Ready node. They were left
out now because every file in this directory is unrun, and shipping a
twelve-manifest bundle that nobody has executed would overstate how much of this
is understood.

## Two open questions this directory depends on

1. **Can the `infraReady` hook still write into `<install-dir>/openshift/`?**
   The hook runs with `OPENSHIFT_INSTALL_MANIFEST_DIR` set, but whether the
   installer has already consumed and removed that tree by then is **not
   verified**. If it has, the Secrets must instead be `oc apply`-ed from the
   `postProvision` hook against `$OPENSHIFT_INSTALL_KUBECONFIG` — which works,
   but means the CCM crash-loops until then and nodes stay tainted meanwhile.
   One experiment settles this and it costs nothing.
2. **Does the node name match the OCI instance display name?** Oracle ships an
   `oci-hostname-update.service` that sets `/etc/hostname` from the instance's
   `displayName`, and this example deliberately does not copy it. With
   `providerID` set explicitly the CCM should not need a name match — but
   "should not" is not "does not".
