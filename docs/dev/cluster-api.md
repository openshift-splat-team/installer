# Cluster API

The installer uses Cluster API controllers through a local control plane powered by `kube-apiserver` and `etcd` running locally.

### Local control plane

The local control plane is setup using the previously available work done in Controller Runtime through [envtest](https://github.com/kubernetes-sigs/controller-runtime/tree/main/tools/setup-envtest). Envtest was born due to a necessity to run integration tests for controllers against a real API server, register webhooks (conversion, admission, validation), and managing the lifecycle of Custom Resource Definitions.

Over time, `envtest` matured in a way that now can be used to run controllers in a local environment, reducing or eliminating the need for a full Kubernetes cluster to run controllers.

At a high level, the local control plane is responsible for:
- Setting up certificates for the apiserver and etcd.
- Running (and cleaning up, on shutdown) the local control plane components.
- Installing any required component, like Custom Resource Definitions (CRDs)
    - For Cluster API core the CRDs are stored in `data/data/cluster-api/core-components.yaml`.
    - Infrastructure providers are expected to store their components in `data/data/cluster-api/<name>-infrastructure-components.yaml`
- Upon install, the local control plane takes care of modifying any webhook (conversion, admission, validation) to point to the `host:post` combination assigned.
    - Each controller manager will have its own `host:port` combination assigned.
    - Certificates are generated and injected in the server, and the client certs in the api-server webhook configuration.
- For each process that the local control plane manages, a health check (ping to `/healthz`) is required to pass similarly how, when running in a Deployment, a health probe is configured.

### Build and packaging

The Cluster API system is formed of a set of binaries. The core Cluster API manager, and the infrastructure providers are built using Go Modules in the `cluster-api` folder.

The binaries are built and packaged during the standard installer build process, `hack/build.sh`. Cluster API specific build flow is contained in the `hack/build-cluster-api.sh` script:
- Builds (as needed) every binary listed as a Go Module in  the `cluster-api` folder.
- Downloads (as needed) the specified version of `envtest` to package `kube-apiserver` and `etcd`.
- Produces a single `cluster-api.zip` file which is then copied in `pkg/clusterapi/mirror`.

To build an `openshift-install` binary with Cluster API bundled:
- Optionally `export SKIP_TERRAFORM=y` if you don't need to use Terraform.
- Run `./hack/build.sh`, the binary is then produced in `bin/openshift-install`.

### Overriding provider artifacts during development

Iterating on an infrastructure provider normally means rebuilding the whole installer, because
the provider binaries and component manifests are baked into `bin/openshift-install`. Two
environment variables let you point a single provider at artifacts on disk instead:

```sh
OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_<NAME>_BINARY=/path/to/controller
OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_<NAME>_COMPONENTS=/path/to/components.yaml
```

**These are a development aid and are not a supported installation interface.** They have no
install-config equivalent, they are not versioned, using them logs a warning, and they may be
changed or removed at any time. Do not use them on a cluster you care about.

`<NAME>` is the provider name upper-cased, with `-` replaced by `_`. The variables are read
per provider, so overriding one leaves the others on their embedded copies, and each of the
two variables is independent — setting only `_BINARY` keeps the embedded component manifest.

Valid names, from the infrastructure providers declared in `pkg/clusterapi/providers.go`:

| Provider | Variable infix |
| --- | --- |
| `aws` | `AWS` |
| `azure` | `AZURE` |
| `azureaso` | `AZUREASO` |
| `azurestack` | `AZURESTACK` |
| `gcp` | `GCP` |
| `ibmcloud` | `IBMCLOUD` |
| `nutanix` | `NUTANIX` |
| `openstack` | `OPENSTACK` |
| `openstackorc` | `OPENSTACKORC` |
| `vsphere` | `VSPHERE` |

Two caveats about which names work:

- **The core Cluster API controller and the envtest binaries cannot be overridden.** They are
  unpacked before the provider machinery runs and never pass through the override lookup, so
  `..._CLUSTER_API_BINARY` and `..._ENVTEST_BINARY` are accepted by your shell and then
  **silently ignored** — no warning, no error. To change those, rebuild.
- **`IBMCLOUD` also applies to Power VS installs**, since both platforms are served by the
  `ibmcloud` provider.

What is validated, and what is not:

- The binary must be an existing, non-empty, executable regular file. If it is an ELF binary
  it must match the machine type, word size and byte order of the running installer; non-ELF
  executables such as shell wrappers are accepted without an architecture check, but a file
  that *looks* like an ELF binary and cannot be parsed is rejected rather than passed through
  to `exec`.
- The components path may be either a single manifest file or a directory of manifests —
  envtest reads both, though it does not recurse into subdirectories.
- Paths are resolved to absolute form before use, so a bare filename refers to the file in
  your working directory rather than to something found on `$PATH`.
- Validation happens when the controller is started, which is **after** the local control
  plane is running and, on most platforms, after the pre-provisioning hook has already created
  cloud resources. A bad path fails the install; it does not prevent that earlier work. These
  variables are a debugging aid, not a safety check.

The path you supply and the SHA-256 of the binary are written to the installer log, so that a
run can be traced back to an exact build. File contents are never logged. The path itself is
logged verbatim, so avoid pointing these variables at locations you would rather not see in a
support bundle.

### Testing the override

The useful loop here needs no cloud account, no credentials and no pull secret. The local
control plane is an envtest `etcd` and `kube-apiserver` started on your own machine, so a
provider controller can be launched, watched and torn down without contacting anything.

Nothing these tests create reaches an installed cluster. The local control plane is temporary
and dies with the test.

#### Prerequisite: populate the mirror

`pkg/clusterapi/mirror/` is committed with only a `README`, so there is no embedded provider
to compare against until a build produces the zip:

```sh
./hack/build.sh
unzip -l pkg/clusterapi/mirror/cluster-api.zip | tail -3
```

Expect roughly 375 MB and 13 entries — the core `cluster-api` binary, ten providers, `etcd`
and `kube-apiserver`. This takes several minutes and downloads the envtest binaries from a
GitHub release, so the first build needs network access. Tests that need the mirror skip
themselves when it is absent and name this script in the skip message; a missing artifact is
a skip, not a failure.

#### Unit tests

```sh
IS_CONTAINER=TRUE go test -short ./pkg/clusterapi/...
```

These cover path resolution, the validation rules and the architecture check in isolation.
They never start a control plane, so they say nothing about whether the override reaches
`exec` — that is what the integration test is for. `hack/go-test.sh` runs the same thing
inside podman and also passes `-short`.

#### Integration test: the real local control plane

```sh
go test -count=1 -p 1 -parallel 1 -timeout 0 -run .Integration ./pkg/clusterapi/...
```

About 15 seconds. Two arms, and the second matters as much as the first:

- **Override arm** — points `..._AWS_BINARY` at a stub, starts it through the installer's own
  controller startup path, and asserts the process was launched from the override and that the
  embedded binary was never unpacked into the bin directory.
- **Embedded arm** — sets no variables, runs the *real* embedded provider with the production
  argument list, and waits for its `/healthz` to answer. This is the regression guarantee: it
  is the evidence that the override changes nothing when you are not using it.

**Do not reach for `hack/go-integration-test.sh` to run just these.** Its package list is
hardcoded and `"$@"` is appended rather than substituted, so passing `./pkg/clusterapi/...`
narrows nothing — it is already inside `./pkg/...`. You get the whole integration suite,
including `cmd/openshift-install`'s `TestAgentIntegration`, which needs `oc`, `nmstatectl` and
`registry.ci.openshift.org` credentials and fails without them. Use the `go test` invocation
above while iterating; the script is what CI runs.

#### Trying it by hand

To watch the mechanism rather than assert on it, any executable will do — a shell wrapper is
accepted, because the architecture check only applies to ELF files:

```sh
printf '#!/bin/sh\nexec sleep 300\n' > /tmp/fake-capa && chmod 0700 /tmp/fake-capa
export OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_AWS_BINARY=/tmp/fake-capa
```

The log then carries the path and its SHA-256 alongside the unsupported-configuration
warning, which is how you confirm the resolver picked up what you meant.

#### Against a real install

This one does contact the cloud, costs money and needs credentials and a pull secret, so it
is not part of the development loop:

```sh
make -C cluster-api bin/linux_amd64/cluster-api-provider-aws
export OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_AWS_BINARY="${PWD}/cluster-api/bin/linux_amd64/cluster-api-provider-aws"
./bin/openshift-install create cluster --dir ./demo
```

Remember the ordering caveat above: by the time the override is validated, pre-provisioning
has already created cloud resources, so a typo here still leaves you with something to destroy.
