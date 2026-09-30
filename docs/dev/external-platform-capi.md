# Provisioning `platform: external` with a user-supplied Cluster API provider

**Status: pilot. Not a supported configuration.** This path is under development. It has
not completed an end-to-end install, it cannot destroy what it creates, and the
install-config fields described here may change without a deprecation cycle.

## What this is

`platform: external` has meant "I provision the infrastructure myself, out of band". This
adds an opt-in: point the installer at a Cluster API infrastructure provider it was never
compiled against — a binary and its CRDs — and let that provider create the
infrastructure.

The installer does not know the provider. It does not import its Go types, does not
generate its manifests, and never calls its cloud. It starts the provider's controller
against its temporary local Cluster API control plane, creates the objects you wrote, and
waits for the `Cluster` to report its infrastructure ready. Everything platform-specific
belongs to the provider.

Nothing here is specific to any one cloud. The examples use the Cluster API Provider for
AWS (CAPA) because it is convenient to build and test against — it is a **reference
provider used to exercise this path**, not an AWS integration. `platform: aws` is
untouched and goes nowhere near this code.

## Before you start: there is no destroy

Read this first, because it costs money.

`openshift-install destroy cluster` does **not** remove infrastructure created this way.
There is no destroy implementation for this platform and no entry in the destroy
registry, so the command will remove nothing and tell you it succeeded at removing
nothing. `openshift-install destroy bootstrap`, run as its own process, fails before it
can delete the bootstrap machine.

**Everything you create here, you remove by hand** — through your cloud provider's
console or CLI, or by running the provider's controller yourself against a real
management cluster. An infrastructure-only run on AWS leaves a VPC, subnets, NAT gateways
and elastic IPs. NAT gateways and elastic IPs bill hourly whether or not anything uses
them.

Destroy is designed and sequenced for the increment after this one. It will be a hook, on
the same principle as provisioning: you supply the method, the installer invokes it.

## Prerequisites

- A provider controller binary built for this host's architecture, and the provider's CRDs
  and component manifests. Anything the provider needs served by the local control plane
  must be in the components — the installer supplies none of it. The fastest way to get a
  matched pair is to extract one the installer already ships; see below.
- Cloud credentials for the provider, in whatever form the provider expects. The
  installer does not read, validate, forward or log them.
- A pull secret, as for any install.

## Getting a provider: extract the one the installer ships

Start here, before pointing this at a provider you built yourself.

```sh
openshift-install extract cluster-api aws --dest-dir=/tmp/capa-artifacts
```

This hidden command writes an embedded infrastructure provider's controller binary and its
component manifests out to a directory, and prints the `clusterAPI:` block that points at
them, indented ready to paste under `platform.external`. It is a fragment, not a whole
`platform:` block: your install-config already has one, and the installer's parser rejects
a duplicate key rather than merging.

Pick a `--dest-dir` outside the install directory. `<install-dir>/cluster-api/` in
particular is where the installer unpacks its own binaries and it is removed wholesale at
teardown.

**Why to prefer it while this path is under development.** A failure with a provider built
elsewhere has two candidate causes — this code, or that build — and separating them is
most of the debugging. The extracted artifacts are the same ones an integrated install of
that provider uses, so anything that goes wrong is attributable to this path. Once it
works with an extracted provider, moving to your own build tests one thing at a time.

`extract cluster-api` with no valid provider name lists what this binary embeds. Only
infrastructure providers are listed: the core Cluster API controller and the envtest
binaries are unpacked by the installer on every run, for every platform, so you never
supply them.

Both files are needed. The binary alone cannot start, because the provider's CRDs have to
reach the local control plane first.

To build a provider yourself instead — for CAPA:

```sh
make -C /path/to/cluster-api-provider-aws managers
```

## Install directory layout

```
<install-dir>/
  install-config.yaml
  external-install/            # you write these
    cluster.yaml
    infrastructure.yaml
    machines/
      bootstrap.yaml
```

`external-install` is a constant and is provisional; it may change before this lands.

**`external-install/` is the only directory you put Cluster API objects in.** The file
names above are examples — the installer reads whatever is there, so organise them as you
like. Every object in it is applied by the installer; there is nothing to apply by hand.

It is deliberately **not** `<install-dir>/cluster-api/`, which the installer uses for the
platforms it generates manifests for. That directory is an asset load path, and the asset
store discards assets whose dependencies are dirty — files placed there alongside an
unconsumed `install-config.yaml` are thrown away with a single warning. It is also where
the installer unpacks the Cluster API and envtest binaries while it runs. It is scratch
space, not an interface.

Files directly in `external-install/` are infrastructure objects, created first. Files in
`external-install/machines/` are machines, created after the infrastructure reports
ready. Only `.yaml`, `.yml` and `.json` are read; other files and other subdirectories
are ignored, so keep notes there if you like. A file may contain several YAML documents
separated by `---`; each is read.

You do not need to create the namespace. The installer supplies it and creates everything
into it.

### Not `manifests/`, and not `openshift/`

These are **not** extra manifests. `<install-dir>/manifests/` and `<install-dir>/openshift/`
hold OpenShift resources destined for **the cluster being installed**; they are baked into
the bootstrap ignition and applied by the bootstrap process once that cluster's API server
is up.

`external-install/` is a different mechanism with a different destination. Its contents go
to the installer's **temporary local Cluster API control plane** — an `etcd` and
`kube-apiserver` running on your machine for the duration of the command — which is where
your provider's controller is watching. Nothing in it is applied to the cluster being
installed, and nothing in it survives the command.

Putting a `Cluster` or an infrastructure CR in `manifests/` does not work: it would be
sent to the installed cluster, which has neither the Cluster API CRDs nor a controller to
reconcile them, and the provider that was supposed to act on it would never see it. The
installer would also report no manifests in `external-install/` and refuse to start.

## install-config.yaml

You must write this file yourself. `external` is a hidden platform — it is not offered by
the interactive wizard — so `openshift-install create install-config` on an empty
directory cannot produce it. `extract cluster-api` prints the `clusterAPI:` block; the rest
is below. Scaffolding this from the survey is planned for a later increment.

The paths below are the ones `extract cluster-api aws --dest-dir=/tmp/capa-artifacts`
writes.

```yaml
apiVersion: v1
metadata:
  name: capi-ext
baseDomain: example.com
platform:
  external:
    platformName: aws
    cloudControllerManager: External
    clusterAPI:
      name: aws
      binaryPath: /tmp/capa-artifacts/cluster-api-provider-aws
      componentsPath: /tmp/capa-artifacts/aws-infrastructure-components.yaml
      # args are appended to the arguments the installer supplies.
      args: []
controlPlane:
  name: master
  replicas: 3
compute:
- name: worker
  replicas: 0
networking:
  networkType: OVNKubernetes
  machineNetwork:
  - cidr: 10.0.0.0/16
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
pullSecret: '{"auths":{...}}'
sshKey: |
  ssh-ed25519 AAAA... user@host
```

Notes on the fields that are not obvious:

- `platformName` is informational only — it is reported, never used for decisions. It is
  unrelated to `clusterAPI.name`, which selects the controller name in logs and the
  developer override environment variables.
- `cloudControllerManager: External` is what you want for infrastructure provisioned by a
  cloud provider, but nothing delivers a CCM to the installed cluster yet, so nodes would
  not initialise. It makes no difference to an infrastructure-only run.
- `compute.replicas: 0` because the installer generates no machines for this platform;
  any machines come from `external-install/machines/`.
- `machineNetwork` should match the network your infrastructure object creates.
- `sshKey` must be a real public key — it is parsed and rejected if malformed.

`platform.external` **without** a `clusterAPI` block behaves exactly as it always has:
self-managed infrastructure, same error if you ask the installer to provision. The
`clusterAPI` block is the whole opt-in.

`binaryPath` and `componentsPath` are local filesystem paths. Remote references — git,
images, digest pinning — and the air-gapped flow are deferred. Paths are resolved to
absolute, the binary is checked for being executable and built for this architecture, and
its SHA-256 is logged so a run can be traced back to an exact build. **File contents are
never logged; only the path, the source and the digest.**

## The manifests

Write these into `<install-dir>/external-install/`, in any file names you like. **You
never apply them yourself** — see [Who applies them, and
when](#who-applies-them-and-when) below.

The `Cluster` is the one object the installer relies on. It waits on
`status.infrastructureReady` to know when the infrastructure is done, and reads
`spec.controlPlaneEndpoint` afterwards. Write it as Cluster API documents it:

```yaml
# <install-dir>/external-install/cluster.yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: example
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
    kind: SomeCloudCluster        # placeholder — your provider's kind goes here
    name: example
```

`SomeCloudCluster` is **not a real kind**. It stands in for whatever your provider
reconciles, because nothing on this path is specific to one cloud. For CAPA it is
`AWSCluster`; see the worked example below.

`apiVersion: cluster.x-k8s.io/v1beta1` on the `Cluster` is **required, and it is not the
version upstream Cluster API documents today.** Provisioning finds the `Cluster` and the
`Machine`s by their compiled-in Go type, so a core object at any other version is created
and then never recognised — the readiness wait would find nothing to wait for and report
the infrastructure ready immediately. The installer rejects the skew up front and names
the version to use. This applies only to `cluster.x-k8s.io` objects; your provider's own
version is its business and is not constrained.

Leave `metadata.namespace` out. The installer sets it on every object it creates, so a
namespace you set here is overwritten rather than honoured.

The infrastructure object is your provider's own API and the installer does not validate
it — it does not know the schema. Consult your provider's documentation. It can go in the
same file as the `Cluster`, separated by `---`, or in its own.

### Worked example: CAPA

With the artifacts from `extract cluster-api aws`, the pair looks like this. Check the
kind, versions and fields against the components file you extracted rather than against
this document — it is the CRD that decides, and the version below is what
`aws-infrastructure-components.yaml` currently serves.

```yaml
# <install-dir>/external-install/cluster.yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: example
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
    kind: AWSCluster
    name: example
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSCluster
metadata:
  name: example
spec:
  region: us-east-1
```

`AWSCluster` serves only `v1beta2` in the shipped components — its `v1beta1` is present
in the CRD but `served: false`, so a manifest written against it is rejected by the API
server, not silently converted.

`spec.region` is the minimum CAPA needs to do anything; everything else it defaults,
including creating a VPC. To place the cluster in an existing VPC, or to control subnets,
CIDRs or the load balancer, consult CAPA's own documentation — those fields are its API,
not the installer's.

The installer checks only that you supplied a `Cluster` and at least one non-namespace
infrastructure object, because the alternative — provisioning nothing and reporting
success — is the worst thing this path could do.

### Who applies them, and when

The installer does, to its local control plane. The order matters, because the local
control plane does not exist when your files are read:

1. **Your files are read from disk** and decoded into objects, before anything starts.
   A malformed manifest, a missing `Cluster` or an absent `external-install/` directory
   fails here — with nothing running and nothing provisioned.
2. **The local control plane starts** — `etcd` and `kube-apiserver` via envtest, on your
   machine.
3. **Your provider's CRDs are installed** into it, from the `componentsPath` you gave in
   the install-config, and your provider's controller is started against it. This is what
   makes an object whose kind this installer was never compiled against servable.
4. **The installer creates your objects** in that control plane: the namespace first, then
   everything directly in `external-install/`.
5. **Your controller reconciles them** and creates the real cloud infrastructure. The
   installer just watches `Cluster.status.infrastructureReady`.
6. Once ready, the infrastructure-ready hook runs and the objects in
   `external-install/machines/` are created the same way.
7. **The local control plane is torn down** when the command ends. Everything in it goes
   with it — the objects you wrote, their status, and any Secret the provider created
   there. The cloud infrastructure your provider created remains, which is why the
   [no-destroy warning](#before-you-start-there-is-no-destroy) above matters.

So the CRDs are the fix, and they arrive at step 3 from `componentsPath`. That is the
whole reason the install-config needs both a binary and a components path: the binary
alone would have nothing to reconcile, because its types would not be served.

Because reading happens at step 1 and creating at step 4, editing files in
`external-install/` while the installer is running has no effect on that run.

## Infrastructure-only runs

Bringing up a new provider is iterative, and nearly all of that iteration is in the
network. Stop before any machine is created:

```sh
OPENSHIFT_INSTALL_INFRASTRUCTURE_ONLY=true openshift-install create cluster --dir=<dir> --log-level=debug
```

The installer creates the infrastructure, waits for the `Cluster` to report ready, runs
the infrastructure-ready hook, then stops and **fails** — no machines, no cluster, a
non-zero exit. That failure is the point. The provisioned infrastructure is left in place
for you to inspect, and it is yours to remove.

You can also leave out `external-install/machines/` entirely; the installer warns rather
than refusing to start.

Each attempt then costs a network rather than a control plane.

## Developer artifact overrides

The environment-variable overrides predate the install-config fields and are kept
permanently as a developer aid. An override for a provider name takes precedence over the
install-config, so you can swap a freshly built binary without editing anything:

```sh
export OPENSHIFT_INSTALL_CLUSTER_API_MYPROVIDER_BINARY=/path/to/new/build
export OPENSHIFT_INSTALL_CLUSTER_API_MYPROVIDER_COMPONENTS=/path/to/components
```

They are validated identically to the install-config paths, so the two cannot disagree
about what is acceptable.

## Controller arguments, and a known limitation

The installer starts the controller with a small argument set it assumes every Cluster
API provider accepts:

```
-v=2
--health-addr=<host:port>
--webhook-port=<port>
--webhook-cert-dir=<dir>
--kubeconfig=<path>
```

**This set is not actually universal, and `args` cannot fix it.** `--health-addr` is
CAPA's spelling; CAPOCI and controller-runtime's own scaffolding use
`--health-probe-bind-address`. Providers parse with `pflag`, which rejects unknown flags
and exits, so a provider using the other spelling dies at startup. `args` is **appended**
to the list above, so you can add a flag but cannot remove or override one the installer
set.

Resolving this — most likely by letting `args` override an installer-supplied flag by
name — is required before a second provider can work, and is deferred to that increment.
For now this path works with providers that accept `--health-addr`.

If the controller exits immediately, run with `--log-level=debug`: controller output is
only shown at debug level, and a flag-parsing failure is visible there.

## Local control plane, and what it is not

The Cluster API control plane the installer runs is envtest — a local `etcd` and
`kube-apiserver` on your machine. It is temporary and it is torn down when the command
ends.

**Nothing in it reaches the cluster being installed.** That applies especially to
Secrets: a credential the provider creates or reads in the local control plane is local
to your machine and to that run. Delivering anything to the installed cluster — a cloud
controller manager, its configuration — is a separate mechanism and is not implemented
on this path yet.

## What does not work yet

- **Destroy**, in either form. See the warning above.
- **Cloud controller manager delivery.** The infrastructure-ready hook exists but the
  External provider does not implement it, so nothing is delivered to the installed
  cluster. A cluster that reached the end would have no CCM and its nodes would not
  initialise.
- **A second provider**, because of the argument-set limitation above.
- **Remote or air-gapped artifacts.** Local filesystem paths only.
- **The interactive wizard.** `create install-config` neither offers this platform nor
  scaffolds `external-install/`. Write both by hand for now.
- **A feature gate.** This is ungated for now; gating is deferred until after the pilot.
