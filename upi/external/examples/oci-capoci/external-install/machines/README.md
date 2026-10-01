# Machine manifests — the three-phase workflow

**Status: never run.** Written 2026-09-30 against `cluster-api-provider-oci`
with no OCI account available. The aws-capa equivalent of this directory was
developed over a dozen installs; this one has executed zero.

## Why this directory exists at all

On an integrated platform the installer generates machine manifests itself —
`pkg/asset/machines/clusterapi.go` builds the `Machine` and infrastructure
machine objects from the install-config's `controlPlane` and `compute` pools.
For `platform: external` it generates nothing, by design: the installer has no
`OCIMachine` type and must never acquire one.

So the machines are supplied. These files are copied into
`<install-dir>/external-install/` before `create cluster`, the installer loads
them as unstructured objects and applies them to its local control plane, and
CAPOCI reconciles them into OCI instances.

| File | Objects | Required? |
| --- | --- | --- |
| `10_bootstrap.yaml` | 1 `Machine` + 1 `OCIMachine` | yes |
| `20_master.yaml` | 3 `Machine` + 3 `OCIMachine` | yes |
| `30_worker.yaml` | 2 `Machine` + 2 `OCIMachine` | no — delete for a compact cluster |

## The substitution contract

Three tokens appear across every file in this directory and in `../cluster.yaml`.
`../../scripts/run-create-command.sh` rewrites all three; nothing else does.

| Token | Replaced with | Known when |
| --- | --- | --- |
| `CLUSTER-ID` | the run's infrastructure ID, e.g. `mrb-oci1-knzjv` | after `create manifests` — read from `metadata.json` |
| `COMPARTMENT-OCID` | `$OCI_COMPARTMENT_ID` | before the run |
| `IMAGE-OCID` | the imported RHCOS custom image OCID | before the run, once — see `../../docs/boot-image.md` |
| `REGION`, `CLUSTERDNS` | `../cluster.yaml` only | before the run |

`CLUSTER-ID` is the infrastructure ID, **not** the cluster name. The installer
appends a five-character random suffix at `create manifests` time, and the
`Cluster` object's name must match it or the `Machine` objects' `clusterName`
will not resolve.

There is deliberately **no token for a subnet or NSG OCID**. `OCIMachine`'s
`networkDetails` accepts `subnetName` and `nsgNames` alongside `subnetId` and
`nsgIds` (`api/v1beta2/types.go`), so machines reference the network CAPOCI
builds by name. Nothing in this directory needs rewriting after the network
exists — which is why there is no fourth phase.

## The three phases

This is the part with no aws-capa equivalent. On AWS the whole install is
`create manifests` then `create cluster`, because CAPA uploads the oversized
bootstrap ignition to S3 itself (`s3Bucket` plus
`ignition.storageType: ClusterObjectStore`). **CAPOCI has no object-store
offload at all** — no object-storage field in `api/v1beta2/`, no `objectstorage`
client under `cloud/services/` — and OCI caps instance metadata at 32,000 bytes
total, which the bootstrap ignition exceeds by two orders of magnitude.

So the offload is done outside the installer, in the gap between two existing
commands:

```
  1. openshift-install create manifests
         → metadata.json exists; CLUSTER-ID is now known
  2. substitute tokens; copy this directory into <install-dir>/external-install/
  3. openshift-install create ignition-configs
         → bootstrap.ign (hundreds of KiB), master.ign, worker.ign (~1.7 KiB)
  4. upload bootstrap.ign to OCI Object Storage; mint a pre-authenticated
     request over it
  5. OVERWRITE <install-dir>/bootstrap.ign with a ~300-byte pointer config
     aimed at that URL
  6. openshift-install create cluster
         → the installer RELOADS bootstrap.ign from disk and publishes the
           stub as the bootstrap machine's ignition Secret
```

Step 5 works because `bootstrap.Bootstrap` is a `WritableAsset` that implements
`Load()` for `bootstrap.ign`
(`pkg/asset/ignition/bootstrap/bootstrap.go:10,18,39-41`) and sits in the
IgnitionConfigs target (`pkg/asset/targets/targets.go:56-64`). The installer
prefers what is on disk over what it would regenerate. **No installer change.**

Full reasoning, the pointer config's exact content, and the security properties
of a pre-authenticated request are in `../../docs/ignition-delivery.md`. Read
that before running step 4 — the URL is an unauthenticated credential over the
cluster's day-0 secrets and must never be logged.

## What creates the ignition Secrets

Not these files. The installer publishes exactly three Secrets into
`openshift-cluster-api-guests`, named `<infraID>-bootstrap`, `<infraID>-master`
and `<infraID>-worker`, from
`pkg/infrastructure/external/clusterapi/ignition.go`, after `InfraReady` and
before the machines are created (`pkg/infrastructure/clusterapi/clusterapi.go`
— `Ignition()` at :361, Secrets created at :372).

Ignition Secrets are **per role, not per machine**. The three master machines
share `CLUSTER-ID-master`; there is no per-machine variant and no way to ask for
one. CAPOCI reads whichever Secret the `Machine` names, verbatim:
`GetBootstrapData()` returns `secret.Data["value"]` unchanged
(`cloud/scope/machine.go:685-702`), then base64-encodes it into
`metadata["user_data"]` (`:353`). It has no opinion about the format — the
string "ignition" does not appear anywhere in CAPOCI's Go source, and unlike
CAPA there is no feature gate to enable.

One consequence worth knowing: `metadata["user_data"]` is **overwritten**, not
merged. A `metadata:` map on the `OCIMachine` is merged first (`:351`) and then
`user_data` is replaced, so there is no way to smuggle extra userdata past the
bootstrap Secret. That is why every machine here carries `metadata: {}`.

## Destroy coverage

The standing rule on this pilot is that every resource created ships with its
teardown in the same change. For this directory:

| Created | Removed by |
| --- | --- |
| the OCI instances | CAPOCI, on `Cluster` deletion — `destroy cluster` |
| the bootstrap ignition object and its pre-authenticated request | `../hooks/infra-hook.sh`, `pre-destroy` |
| the object storage bucket | **not** removed — reused across runs, created once |

The bucket is deliberately long-lived and deliberately private. Only the object
and the pre-authenticated request over it are per-cluster.

## Before the first run

In rough order of how likely each is to stop the install:

1. **No RHCOS image for OCI is published, so one must be built.** `IMAGE-OCID`
   has nothing to point at until it is. The capability is there — the RHCOS in
   the release payload ships Ignition 2.26.0 with the `oraclecloud` provider
   compiled in, measured 2026-10-01 — so the work is importing the published
   `qemu` QCOW2 with `ignition.platform.id=oraclecloud` written into it.
   `../../docs/boot-image.md` has the measurement, the artifact URL and what
   is still unproven.
2. **22623 has no listener until the hook adds one.** `../cluster.yaml`
   explains why CAPOCI cannot express it and why an out-of-band listener
   survives reconcile; `../../docs/capi-requirements.md` §8 has the open
   question about populating its backends in time for the masters' first boot.
3. **Worker CSRs need manual approval.** See the header of `30_worker.yaml`.
4. **The subnet CIDRs here are wider than CAPOCI's defaults.** The default
   control-plane subnet is a `/29` — five usable addresses for three masters
   plus bootstrap. `../cluster.yaml` widens it and says so.
