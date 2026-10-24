package clusterapi

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/openshift/installer/pkg/asset/installconfig"
	infracapi "github.com/openshift/installer/pkg/infrastructure/clusterapi"
	"github.com/openshift/installer/pkg/infrastructure/external/hooks"
)

// PostProvision runs the user's post-provision hook, if they configured one.
//
// This is the second and last point at which an External install can call out
// to the partner, and it exists for what InfraReady structurally cannot do.
// InfraReady runs before any machine is created, so everything it can name is
// something the infrastructure provider created: a load balancer, a network,
// a subnet. Anything the *cluster* creates for itself is not there yet and
// cannot be, which is why the ingress wildcard has no answer at that point.
//
// On this platform the cluster's external ingress address is created by the
// partner's own cloud controller manager, in response to a Service the
// cluster applies to itself during bootstrap -- the ingress operator selects
// HostNetwork here and publishes no load balancer of its own, so nothing else
// will ask. That address is therefore assigned minutes after InfraReady has
// returned, and `*.apps` cannot point at it until it exists.
//
// PostProvision is where it does exist. The installer calls it once the
// control-plane machines have been created
// (pkg/infrastructure/clusterapi/clusterapi.go:449), which is after the
// cluster's own API is serving and before `wait-for bootstrap-complete`. A
// hook here can wait for the address and publish it, and the operators that
// need `*.apps` -- authentication and console -- are not required for
// bootstrap to complete, so waiting here does not deadlock against the thing
// it is waiting for.
//
// The waiting is the hook's job, not the installer's. How long a cloud takes
// to allocate an address, and what "ready" means for it, is exactly the
// provider knowledge this platform is built to not have.
func (p Provider) PostProvision(ctx context.Context, in infracapi.PostProvisionInput) error {
	configured := configuredHooks(in.InstallConfig)
	if configured == nil || configured.PostProvision == nil {
		warnNoPostProvisionHook(in.InstallConfig)
		return nil
	}

	req, err := buildRequest(ctx, in.Client, hooks.PostProvision, configured.PostProvision, in.InstallConfig, in.InfraID)
	if err != nil {
		return err
	}
	return hooks.Run(ctx, req)
}

// warnNoPostProvisionHook names the consequence rather than the omission.
//
// As with InfraReady, running no hook is supported: a cluster published
// Internal-only, or one whose ingress is fronted out of band, needs nothing
// here. But the failure when nothing creates the wildcard arrives an hour
// later as two degraded operators complaining about a hostname, so the
// warning quotes the hostname they will complain about.
func warnNoPostProvisionHook(ic *installconfig.InstallConfig) {
	domain := "the cluster domain"
	if ic != nil && ic.Config != nil {
		domain = ic.Config.ClusterDomain()
	}
	logrus.Warnf("No platform.external.clusterAPI.hooks.postProvision is configured, so the installer is "+
		"creating no ingress DNS for this cluster. If nothing else creates a wildcard record for *.apps.%s, "+
		"the cluster will reach the end of bootstrap and then stop short of complete: the authentication and "+
		"console operators resolve oauth-openshift.apps.%s and will report it as no such host.", domain, domain)
}
