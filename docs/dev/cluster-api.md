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

```sh
make -C cluster-api bin/linux_amd64/cluster-api-provider-aws
export OPENSHIFT_INSTALL_EXPERIMENTAL_CAPI_PROVIDER_AWS_BINARY="${PWD}/cluster-api/bin/linux_amd64/cluster-api-provider-aws"
./bin/openshift-install create cluster --dir ./demo
```
