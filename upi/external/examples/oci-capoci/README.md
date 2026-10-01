# `platform: external` with CAPOCI — the second pilot

Installing OpenShift on Oracle Cloud Infrastructure with `platform: external`,
driving infrastructure through the **Cluster API Provider for OCI** (CAPOCI)
from the installer's own local control plane.

> **STATUS: NOTHING HERE HAS EVER RUN.**
>
> Written 2026-09-30 against local clones of `cluster-api-provider-oci` and
> `oci-openshift`, with no OCI account available. Every claim about CAPOCI,
> Oracle's assets and the installer is cited to `file:line` and was checked
> against the clone. Every claim about what OCI's API *does* comes from the
> published reference, not from a call.
>
> The [aws-capa](../aws-capa/) sibling of this directory came out of a dozen
> real installs. This one came out of reading. The structure is deliberately
> identical so the differences are the interesting part.

## Why a second provider

The first pilot proved the installer can drive a Cluster API provider it was
never compiled against, using CAPA as the reference. One provider does not
establish a contract. CAPOCI is the test of whether the seams — the artifact
override, the unknown-GVK path, the three hooks, the ignition Secrets — are
actually provider-agnostic or merely CAPA-shaped.

The constraint set by the user: **avoid installer changes.** Where CAPOCI needs
something specific, use the existing hooks. That constraint held, with one
caveat covered below.

## The short version

| Question | Answer |
| --- | --- |
| Does the installer need changes? | **No.** One controller flag mismatch, solved by a wrapper script the installer's own validator explicitly sanctions. |
| Does CAPOCI accept Ignition? | **Yes, with no feature gate** — better than CAPA, which needs `BootstrapFormatIgnition=true`. |
| The VCN-ID problem the user raised? | **The easy one.** Four values, three from `infraReady`, no credentials at all. |
| So what actually blocks it? | **Two things, neither of them CAPI:** no *published* RHCOS boot image for OCI — though the RHCOS we ship can read OCI metadata, measured 2026-10-01 — and OCI's 32,000-byte metadata cap versus the bootstrap ignition. |
| Anything missing in CAPOCI itself? | **One gap:** its API load balancer has exactly one listener, on 6443. OpenShift needs 22623 too. Workable out of band; verified not to be pruned by reconcile. |

## Read in this order

| | |
| --- | --- |
| **[docs/capi-requirements.md](docs/capi-requirements.md)** | **start here.** Eight requirements a CAPI provider must meet to drive a `platform: external` install, each scored against CAPOCI with citations. |
| [docs/boot-image.md](docs/boot-image.md) | work item 1: no RHCOS artifact for OCI is *published*, though Ignition 2.26.0 in our RHCOS has the `oraclecloud` provider compiled in. Three routes, and why route A is now the plan. |
| [docs/ignition-delivery.md](docs/ignition-delivery.md) | blocker 2: the bootstrap ignition does not fit in OCI instance metadata, and CAPOCI has no object-store offload. |
| [scripts/README.md](scripts/README.md) | the flag shim, the three-phase workflow, how to build the artifacts. |
| [external-install/machines/README.md](external-install/machines/README.md) | the substitution contract and destroy coverage. |
| [external-install/extra-manifests/README.md](external-install/extra-manifests/README.md) | the CCM and CSI config, and why it cannot be a day-0 manifest. |

## Layout

```
install-config.yaml              platformName: oci, the clusterAPI block, the three hooks
external-install/
  cluster.yaml                   Cluster + OCIClusterIdentity + OCICluster
  machines/
    10_bootstrap.yaml            1 Machine + OCIMachine
    20_master.yaml               3 Machine + OCIMachine
    30_worker.yaml               2 Machine + OCIMachine  (optional)
  extra-manifests/               day-0 MachineConfigs: kubelet providerID
  hooks/
    infra-hook.sh                infraReady / postProvision / preDestroy
scripts/
  capoci-shim.sh                 the one flag translation
  run-create-command.sh          three-phase driver
  run-destroy-command.sh         teardown + an independent secret-cleanup check
docs/
```

## The two hard problems

Neither is about Cluster API, and neither was the thing anyone expected to be
hard. The first was written as a blocker and was **downgraded to a work item on
2026-10-01**, by measurement.

### 1. No RHCOS boot image for OCI is published — but the capability is there

`data/data/coreos/coreos-rhel-9.json`, `coreos-rhel-10.json` and `scos.json`
list artifacts for aws, azure, azurestack, gcp, ibmcloud, kubevirt, metal,
nvidiabluefield, nutanix, openstack, powervs, qemu, qemu-secex and vmware.
**There is no `oci` key on any architecture in any stream.**
`OCIMachine.spec.imageId` is a bare OCID — unlike CAPA there is no
`imageLookup` filter — so something has to put an image in the tenancy first.

Today's OpenShift-on-OCI is Agent-based or Assisted, with the ignition baked
into a per-cluster boot image
(`oci-openshift/terraform-stacks/shared_modules/compute/main.tf:51-53` sets
`metadata.user_data` to a *bash script*, not an Ignition config). That route
proves nothing about Ignition delivery, which is precisely the thing this pilot
is testing.

**The question that gated everything is answered: yes.** Measured on a running
node of the `mrb-ext14` AWS `platform: external` cluster, built from the same
release payload: RHCOS `10.2.20260918-0` ships **Ignition 2.26.0**, four minor
versions past the 2.22.0 that added `oraclecloud`, and the provider's `init`
and `fetchConfig` Go symbols are present in the shipped binary — so it is
registered, not merely referenced. Afterburn 5.10.0 knows OCI too.

So this is a **packaging gap, not a capability gap**, and route A — import the
published `qemu` QCOW2 and force `ignition.platform.id=oraclecloud` — is the
plan rather than a hope. The narrow thing still unproven is *writing* that
platform ID into an imported image: it is normally baked in at build time by
`coreos-installer install --platform`, and OCI has no kernel-arguments field on
a custom image. Details, the measurement recipe and the artifact URL are in
[docs/boot-image.md](docs/boot-image.md).

### 2. The bootstrap ignition does not fit

OCI caps user data and other metadata at **32,000 bytes total**, about 23,900
raw once base64 inflation is accounted for. Master and worker ignition are
pointer configs of roughly 1.7 KiB and fit easily. The bootstrap ignition is
hundreds of KiB and does not.

Integrated platforms solve this inside the installer — GCP
(`pkg/infrastructure/gcp/clusterapi/clusterapi.go:158`), Azure
(`pkg/infrastructure/azure/azure.go:1119`) and IBM Cloud
(`pkg/infrastructure/ibmcloud/clusterapi/clusterapi.go:464`) all stage the
bootstrap ignition in their own object storage. CAPA solves it in the provider,
with `s3Bucket` plus `ignition.storageType: ClusterObjectStore`. **CAPOCI does
neither** — there is no object-storage field in `api/v1beta2/` and no
`objectstorage` client under `cloud/services/`.

`platform: external` structurally cannot adopt the installer-side answer: it
has no cloud credentials, by design.

The answer that needs no installer change: between `create ignition-configs`
and `create cluster`, upload `bootstrap.ign` to OCI Object Storage, mint a
pre-authenticated request, and overwrite the file on disk with a ~300-byte
pointer config. The installer reloads it, because `bootstrap.Bootstrap`
implements `Load()` and sits in the IgnitionConfigs target. This is the
ordinary UPI pattern. See [docs/ignition-delivery.md](docs/ignition-delivery.md)
— including why the pre-authenticated request URL is itself a credential and
must never be logged.

## The CAPOCI gap: one listener

CAPOCI's API-server load balancer serves exactly one port. Every port in both
reconcilers is `s.APIServerPort()` —
`cloud/scope/network_load_balancer_reconciler.go:55,72,187` and
`cloud/scope/load_balancer_reconciler.go:52,69,151,158` — and there is no
equivalent of CAPA's `controlPlaneLoadBalancer.additionalListeners`. OpenShift
masters fetch their real configuration from `api-int:22623`.

The `infraReady` hook adds a second listener out of band, and that was checked
rather than assumed: `IsNLBEqual` compares only the display name and the health
checker of the single `apiserver-lb-backendset` (`:301-320`), and `UpdateNLB`
sends only `DisplayName` plus that health checker (`:122-150`). The LBaaS path
is weaker still — `IsLBEqual` compares the display name alone (`:250-255`).
Neither enumerates or prunes listeners, so one added by the hook survives.

What is **not** settled is backend population. CAPOCI registers control-plane
machines into `apiserver-lb-backendset` only (`cloud/scope/machine.go:874-895`),
so the hook must populate the 22623 backend set itself — and the masters want
that port during their first boot, which is before `postProvision` runs. That
is the open question in
[docs/capi-requirements.md](docs/capi-requirements.md) §8.

The right long-term fix is upstream in CAPOCI: an `additionalListeners` field
on `apiServerLoadBalancer`, matching what CAPA already has.

## What the hooks do

The installer calls a user-supplied program at three points
(`pkg/infrastructure/clusterapi/clusterapi.go`): `InfraReady` at :312, after
the network exists and before any machine; `PostProvision` at :457, after the
machines; `PreDestroy` during teardown. There is no `preProvision` hook —
`pkg/types/external/platform.go`'s `Hooks` has only those three.

| Hook | Does |
| --- | --- |
| `infraReady` | adds the 22623 listener and an empty backend set; creates the api/api-int DNS; writes the CCM and CSI config Secrets |
| `postProvision` | populates the 22623 backend set; creates the `*.apps` wildcard once the ingress load balancer exists |
| `preDestroy` | removes all of the above, plus the bootstrap ignition object and its pre-authenticated request |

Everything in that table is outside Cluster API's ownership, so deleting the
`Cluster` removes none of it. The standing rule on this pilot is that each
created resource ships with its teardown in the same change.

## The CCM config — the question the user raised

> *"their CCM config/secret requires the VCN ID, which exists only in
> InfraReady, so we may not be possible to inject it in initial ignitions, but
> can apply in infraready phase"*

Correct, and it turns out to be the **easiest** of the problems here, not the
hardest. The OCI CCM and CSI read two Secrets
(`oci-openshift/custom_manifests/manifests/01-oci-driver-configs.yml`) holding
four values between them:

| Field | Source | Day-0? |
| --- | --- | --- |
| `compartment` | user input | yes |
| `vcn` | CAPOCI | no |
| `loadBalancer.subnet1` | CAPOCI | no |
| `loadBalancer.securityLists.<subnet>` | CAPOCI | no |

The three unknowns are all readable at `infraReady` from
`$OPENSHIFT_INSTALL_INFRA_JSON` — the whole `OCICluster` object, handed to the
hook as JSON precisely so the installer never needs a CAPOCI type. Note they
are **spec** paths: CAPOCI writes the OCIDs of what it creates back into the
spec (`cloud/scope/vcn_reconciler.go:43,51`), and `OCIClusterStatus` has no
network block at all.

And **no credential is involved**. Both Secrets set
`useInstancePrincipals: true`; the in-cluster components authenticate as the
instance. No OCI API key ever reaches the installed cluster — a genuine
improvement on the AWS example, which ships CCM credentials as a day-0 Secret.
The API key CAPOCI itself uses lives only in the installer's temporary local
control plane and is destroyed with it.

The one thing still unverified: whether `infraReady` can still write into
`<install-dir>/openshift/`, or whether the Secrets must be `oc apply`-ed at
`postProvision` instead. The hook does both; one experiment settles it and it
costs nothing.

## Before touching an Oracle account

1. **Build the image.** Fetch the `qemu` QCOW2 the installer's own stream
   metadata names, write `ignition.platform.id=oraclecloud` into it, import it,
   boot one instance and confirm `rpm -q ignition` is ≥ 2.22.0 on *that* build
   — the measurement above was taken on the release payload, which is newer
   than the stream artifact. This is the step with real unknowns in it; see
   [docs/boot-image.md](docs/boot-image.md).
2. Build the CAPOCI artifacts and the shim — [scripts/README.md](scripts/README.md).
3. Create the private object storage bucket.
4. Set up the dynamic group and IAM policy for instance principals; the CCM
   needs them and nothing here creates them.
5. Paste the imported image's OCID into the machine manifests.
6. Run `run-create-command.sh <n> infra-only` first. It stops after
   `infraReady`, creating no machines, which exercises the DNS, the 22623
   listener and the CCM Secrets for the price of a VCN and a load balancer. If
   the hook is wrong, that is where it will show, cheaply.

## A note on this directory's name

The convention is `<cloud>-<capi-provider-short-name>`: `aws-capa`,
`oci-capoci`, and `azure-capz` if it ever exists. The request named this
`oracle-capoci`; the convention gives `oci-capoci`, matching the `oci` that
Oracle's own automation puts in `platform.external.platformName`
(`oci-openshift/terraform-stacks/shared_modules/manifest/locals.tf`) and the
`oci` the installer already uses in every CoreOS and cloud identifier.
**Reversible with one `git mv`** — say the word.
