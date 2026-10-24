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

## Before you start: what destroy does and does not cover

Read this first, because it costs money.

`openshift-install destroy cluster` works for this platform, and it works by handing the
cluster back to the provider that built it. The local control plane is gone by then — it
does not survive `create cluster` — so destroy restores the Cluster API objects from
`<install-dir>/.clusterapi_output/`, starts your provider against them, and deletes the
`Cluster`. Deleting a `Cluster` deletes its infrastructure object, and the core controller
holds the `Cluster` open until the provider reports that object gone. **That directory,
and `metadata.json`, are what make the infrastructure removable. Lose the install
directory and you remove it by hand.**

Three things are outside that guarantee, and each of them bills:

- **Anything a hook created.** DNS records, a load balancer the cluster asked its own
  cloud controller manager for — none of it is owned by Cluster API, which is why it
  needed a hook in the first place. Removing it is the `preDestroy` hook's job, and only
  yours will know how. See [Hooks](#hooks-automating-what-cluster-api-cannot-express).
- **Anything the provider did not create**, such as a pre-existing network you supplied.
  That is left in place by design, and destroy says so when it finishes.
- **Machines the installer did not create through Cluster API.** On this platform the
  compute machines come from the machine API once the cluster is up.

`openshift-install destroy bootstrap`, run as its own process, is not exercised on this
platform. `create cluster` removes the bootstrap machine itself at the usual point.

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
      masters.yaml
      workers.yaml             # optional; without it the cluster has no compute
    extra-manifests/           # for the installed cluster, not for CAPI
      99_ccm.yaml
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
ready. Files in `external-install/extra-manifests/` are something else entirely — they go
to the **installed cluster**, and are covered in
[Delivering a cloud controller manager](#delivering-a-cloud-controller-manager). Only
`.yaml`, `.yml` and `.json` are read; other files and other subdirectories are ignored, so
keep notes there if you like. A file may contain several YAML documents separated by
`---`; each is read.

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

The one exception is `external-install/extra-manifests/`, which is for the installed
cluster and is described next. It is a subdirectory of `external-install/` for
housekeeping reasons — one directory to write, one to copy, one to archive — not because
it shares the mechanism.

## Delivering a cloud controller manager

This is the part of `platform: external` that has no default. Setting the platform sets
`status.platformStatus.external.cloudControllerManager.state: External` on the
`Infrastructure` object, which is a declaration that **you** supply the CCM. The installer
does not, and cannot: it does not know your cloud.

It is not optional. The kubelet runs with `--cloud-provider=external`, so every node
registers carrying `node.cloudprovider.kubernetes.io/uninitialized`, and only a CCM
removes it. Until it is removed, nothing schedules, and `create cluster` fails at
`Bootstrap failed to complete` with a healthy API server and a full set of registered,
`NotReady` masters. That failure looks like a bug and is not one.

Put the manifests in `external-install/extra-manifests/`:

```
<install-dir>/external-install/extra-manifests/
  99_ccm-credentials.yaml
  99_ccm.yaml
```

Every `.yaml`, `.yml` or `.json` file there is copied into the generated `openshift/`
manifests, which means:

- `bootkube.sh` moves them in with the rest of the manifests, and `cluster-bootstrap`
  applies them to the bootstrap control plane **before** it waits for the cluster to come
  up. So they are created while the nodes are still tainted, which is the only moment at
  which a CCM is any use.
- A `MachineConfig` placed there is rendered by the bootstrap machine-config server, so it
  reaches the machines on their first boot, without a running Machine Config Operator.

Two things your CCM pod almost certainly needs:

- **Tolerations** for `node.cloudprovider.kubernetes.io/uninitialized` and
  `node.kubernetes.io/not-ready`, and a master node selector. When the pod is created, the
  only nodes that exist are tainted and `NotReady`; a CCM that cannot be scheduled onto one
  of them can never remove the taint that is keeping them that way.
- **A provider ID on each node.** A CCM matches a `Node` to a cloud instance by
  `spec.providerID`, falling back to a lookup by node name — and the fallback is not
  reliable, because the name the kubelet registers need not be the name the cloud knows the
  instance by. Ship a `MachineConfig` that writes `KUBELET_PROVIDERID` into
  `/etc/kubernetes/kubelet-env` from your cloud's metadata service, ordered
  `Before=kubelet.service`. Both the OpenShift CI job for this platform and Oracle's
  OCI assets do exactly this, which is about as much cross-provider agreement as exists.

### Why not just write into `openshift/`?

Because that directory is an asset load path and loading is exclusive. If the asset store
finds any file in `openshift/`, the `Openshift` asset is marked as loaded from disk and its
`Generate` never runs — so one hand-placed file silently replaces every manifest the
installer would have produced. The only safe way to use it is to run `create manifests`,
edit the tree, then `create ignition-configs`, as three separate commands. That works, and
it is what the CI job does today, but it cannot be a single `create cluster`.

`extra-manifests/` is read *during* `Generate` rather than *instead of* it, so it adds
rather than replaces, and one command is enough.

### What the installer does not do

It does not parse, validate, template or reorder anything you put there. It does not know
what your CCM needs and must not acquire an opinion — that coupling is the thing this
platform exists to avoid. Two consequences:

- **Name collisions are refused, not merged.** A file whose name matches one the installer
  generates fails the install with a message naming the file. Prefix yours.
- **Nothing is redacted.** If your CCM needs a cloud credential, it goes in a `Secret` in
  this directory, and from there into the bootstrap ignition and the install directory in
  the clear. Treat the whole install directory as secret material once you do that.

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
      # Optional. Programs the installer runs for you at three points; see Hooks
      # below. Paths are relative to external-install/, so they travel with the
      # install directory.
      hooks:
        infraReady:
          program: hooks/dns.sh
        postProvision:
          program: hooks/dns.sh
          args:
            - --input-service=openshift-ingress/router-external-default
            - --input-dns-zone=Z0123456789ABCDEFGHIJ
        preDestroy:
          program: hooks/dns.sh
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
  any machines come from `external-install/machines/`. On this platform the field does
  **not** create anything, and setting it to 2 does not give you two workers. It controls
  exactly one thing: at zero, `MastersSchedulable` is set true
  (`pkg/asset/manifests/scheduler.go:66-75`) and the control-plane nodes also carry the
  `worker` role, which is what makes a compact three-node install work. See
  [Machines](#machines-bootstrap-control-plane-and-workers) before changing it.
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

## Hooks: automating what Cluster API cannot express

Some of what an install needs is not in the Cluster API contract at all, and DNS is the
case that forces the issue. A control-plane machine fetches its ignition from
`https://api-int.<clusterDomain>:22623/config/master`, yet the core `Cluster` carries one
`spec.controlPlaneEndpoint` and has no field for an internal endpoint. An integrated
platform creates those records in its own `InfraReady`. This platform has no such code
and must not grow any, so it calls out to a program you supply.

There are three points, all optional:

| Hook | When it runs | What it is for |
| --- | --- | --- |
| `infraReady` | after `Cluster.status.infrastructureReady`, before any machine exists | anything that can be named from what your provider built — `api` and `api-int` DNS |
| `postProvision` | after the control-plane machines are created, before `wait-for bootstrap-complete` | anything that can only be named after the cluster starts building it — the `*.apps` wildcard |
| `preDestroy` | during `destroy cluster`, before the `Cluster` is deleted | removing all of the above |

```yaml
platform:
  external:
    clusterAPI:
      hooks:
        infraReady:
          program: hooks/dns.sh
        postProvision:
          program: hooks/dns.sh
          args:
            - --input-service=openshift-ingress/router-external-default
            - --input-dns-zone=Z0123456789ABCDEFGHIJ
        preDestroy:
          program: hooks/dns.sh
```

`program` is a path relative to `external-install/` and must stay inside it: the install
directory is the unit you copy, archive and hand to someone else, and a hook reaching
outside it would run something that did not travel with it. The installer execs it
directly — no shell — so the execute bit and, for a script, a `#!` line are both
required. Its SHA-256 is logged; its contents never are.

### The input contract

The input divides in two, and the division is the point. What the **installer** knows is
environment, identical for every hook: `OPENSHIFT_INSTALL_HOOK` (`infra-ready`,
`post-provision` or `pre-destroy`), `OPENSHIFT_INSTALL_INFRA_ID`,
`OPENSHIFT_INSTALL_CLUSTER_NAME`, `OPENSHIFT_INSTALL_BASE_DOMAIN`,
`OPENSHIFT_INSTALL_CLUSTER_DOMAIN`, `OPENSHIFT_INSTALL_PUBLISH`, `OPENSHIFT_INSTALL_DIR`,
`OPENSHIFT_INSTALL_MANIFEST_DIR`, `OPENSHIFT_INSTALL_STATE_DIR`,
`OPENSHIFT_INSTALL_KUBECONFIG`, `OPENSHIFT_INSTALL_CONTROL_PLANE_ENDPOINT_HOST` and
`…_PORT`, plus `OPENSHIFT_INSTALL_CLUSTER_JSON` and `OPENSHIFT_INSTALL_INFRA_JSON` — file
paths, not contents — holding the core `Cluster` and your own infrastructure object
copied verbatim out of the local control plane.

`OPENSHIFT_INSTALL_INFRA_JSON` is the whole mechanism. The installer cannot read
`status.networkStatus.apiServerElb.dnsName` off an `AWSCluster`, because it has no
`AWSCluster` type and must not acquire one. It can hand you the object and let you dig.

What **you** know is `args`, passed to your program as its argv, in order and verbatim.
The installer does not parse, expand, template or interpret any of it. Use long flags
with explicit values rather than positional arguments: the program that reads them may be
a script today and a compiled binary tomorrow, and a flag survives that change where a
position does not. `args` is recorded in `metadata.json` so `destroy cluster` can run the
teardown half of the same automation — so do not put a credential in one.

`OPENSHIFT_INSTALL_STATE_DIR` is how destroy stays honest. It persists across hook runs
and into `destroy cluster`; anything you create is outside Cluster API's ownership, so
what you write there is the only link between the two directions.

### The output contract

Exit `0` is success. Any other exit fails the install or the destroy — deliberately, and
this is the right strictness: a hook exists because something the cluster needs is not
being created by anything else, so a hook that failed means that thing does not exist.
Warning and continuing is what produces the failure this mechanism was built to avoid, an
install that runs for forty minutes and dies at a bootstrap stage with nothing pointing
back at the cause.

Everything on stdout and stderr is streamed to the installer log as it arrives, prefixed
with the hook kind, and the last few lines are quoted in the installer's own error. Say
the reason last.

### Why `postProvision` exists

Because `infraReady` structurally cannot do ingress. At `infraReady` nothing exists but
what your provider built, and on this platform your provider does not build an ingress
load balancer for anyone: the ingress operator selects
`endpointPublishingStrategy.type: HostNetwork`, so the routers bind ports 80 and 443 on
the nodes and the only Service the operator creates is a `ClusterIP`. Nothing asks for an
external address, so nothing creates one.

The answer is to ask on the cluster's behalf: ship a `Service type=LoadBalancer` through
[`extra-manifests/`](#delivering-a-cloud-controller-manager) and let the cloud controller
manager you already had to supply reconcile it. That address is assigned at runtime,
minutes into bootstrap — long after `infraReady` returned. `postProvision` is the first
point at which it is knowable and still early enough to matter, and waiting there does
not deadlock: the operators that need `*.apps` are `authentication` and `console`, and
neither is required for bootstrap to complete.

How long to wait, and what ready means for your cloud, is your hook's business. The
installer does not poll anything on your behalf.

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

### Machines: bootstrap, control plane and workers

Machines go in `external-install/machines/`, and like everything else here they are
yours to write — `pkg/asset/machines/clusterapi.go` generates none for this platform.

Each `Machine` needs a bootstrap Secret to take its ignition from, and the installer
creates those itself, in its local control plane, named `<infraID>-<role>` for three
roles:

| `spec.bootstrap.dataSecretName` | Contents |
| --- | --- |
| `<infraID>-bootstrap` | the full bootstrap ignition, ~300 KiB |
| `<infraID>-master` | a pointer config aimed at `api-int:22623/config/master` |
| `<infraID>-worker` | a pointer config aimed at `api-int:22623/config/worker` |

You cannot choose these names — `IgnitionSecret`
(`pkg/infrastructure/clusterapi/clusterapi.go:658`) builds them from the infrastructure
ID and the role. And the infrastructure ID is `<metadata.name>-<5 random characters>`,
generated during asset resolution, so it is not knowable when you write the file. That
forces a two-phase workflow:

1. `openshift-install create manifests --dir=<dir>`
2. read `status.infrastructureName` out of
   `<dir>/manifests/cluster-infrastructure-02-config.yml` and substitute it into
   `external-install/`
3. `openshift-install create cluster --dir=<dir>`

Editing between the two commands is safe precisely because `external-install/` is not an
asset load path. Whether the contract should keep requiring this, or the installer should
instead adapt to the names in your manifests, is an open question for the enhancement.

A machine is a control-plane machine if the **core** `Machine` carries the
`cluster.x-k8s.io/control-plane` label; that is how Cluster API's own
`util.IsControlPlaneMachine` reads it, and providers key security-group selection and API
load balancer registration off the same thing. A worker is a machine without it. Nothing
else distinguishes the two, and a worker that is absent from the API target groups is
correct, not broken.

#### Workers are day-0 here, and that is not a stylistic choice

On every integrated platform there are no worker machines in this directory, because
workers arrive *after* bootstrap: the installer generates MachineSets and the cloud's
machine-api actuator reconciles them in-cluster.

Neither exists here. A non-integrated provider ships no machine-api actuator, and the
installer generates no MachineSets for this platform —
`pkg/asset/machines/worker.go:807` is `case externaltypes.Name, nonetypes.Name:` with an
empty body. **So nothing ever creates a worker.** Without worker `Machine`s in this
directory the cluster is permanently the size of its control plane, which is why
`compute.replicas: 0` and schedulable masters are the shape every run so far has used.

Writing them has three consequences worth knowing before you do:

- **They provision before bootstrap.** The wait at
  `pkg/infrastructure/clusterapi/clusterapi.go:405` covers every `Machine` the installer
  created, with no control-plane filter, inside a 15-minute timeout. On every other
  platform a worker cannot delay bootstrap; here a worker that fails to come up fails the
  install. The wait is only for `status.phase` to reach `Provisioned` or `Running` — the
  instance existing, not the node joining — so in practice it adds little time.
- **They boot before the permanent machine config server exists.** This is fine: the
  bootstrap node's MCS serves every pool, and a machine that boots before even that is up
  retries rather than failing. It is the reason day-0 workers are viable at all.
- **Set `compute.replicas` deliberately.** At 0 the masters stay schedulable and these
  machines are extra capacity, so a worker that never joins still leaves a converging
  cluster. At 2 the masters are not schedulable and ingress and console have nowhere to
  run until these machines join — the conventional shape, and the riskier one to try
  first.

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
6. Once ready, the `infraReady` hook runs and the objects in `external-install/machines/`
   are created the same way. This is where DNS for `api` and `api-int` has to come from,
   because the control-plane machines about to boot resolve `api-int` to fetch their
   ignition. **All** the machines here are created and waited for at this point,
   workers included — see [Machines](#machines-bootstrap-control-plane-and-workers).
7. **Once the control-plane machines exist**, the `postProvision` hook runs — still before
   the installer waits for the bootstrap to complete. This is where `*.apps` comes from;
   it cannot come from step 6, because the address it points at does not exist yet.
8. **The local control plane is torn down** when the command ends. Everything in it goes
   with it — the objects you wrote, their status, and any Secret the provider created
   there. The cloud infrastructure your provider created remains, and the only remaining
   record of it is the install directory — see
   [what destroy covers](#before-you-start-what-destroy-does-and-does-not-cover) above.

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
to your machine and to that run. Delivering something to the installed cluster is a
separate mechanism — `external-install/extra-manifests/` for day-0 objects, and a hook
for anything that has to be done against the cluster's own API.

## What does not work yet

- **`destroy bootstrap`** as a standalone command. `destroy cluster` works; see
  [what destroy covers](#before-you-start-what-destroy-does-and-does-not-cover).
- **Anything the installed cluster needs beyond day-0 manifests and a hook.** A CCM is
  delivered through `external-install/extra-manifests/`, described above, and that is the
  whole of the mechanism: files in, manifests out. The installer does not template,
  validate or order what you put there. A hook can reach the cluster's own API once it is
  serving, but the installer runs it and reads its exit status — it does nothing on the
  cluster on the hook's behalf.
- **Day-2 compute.** Day-0 worker `Machine`s remove the size cap, and that is all they
  do. They are created once, by the installer, in a control plane that is destroyed when
  the command ends; nothing reconciles them afterwards. There is no MachineSet, no
  scaling, no replacement of a failed node, and no autoscaling. Closing this needs
  something the installer does not currently do — a machine-api actuator from the
  partner, or Cluster API running in the installed cluster — and neither is in scope
  here. Of everything on this list, it is the largest gap between this path and an
  integrated platform.
- **A second provider**, because of the argument-set limitation above.
- **Remote or air-gapped artifacts.** Local filesystem paths only.
- **The interactive wizard.** `create install-config` neither offers this platform nor
  scaffolds `external-install/`. Write both by hand for now.
- **A feature gate.** This is ungated for now; gating is deferred until after the pilot.
