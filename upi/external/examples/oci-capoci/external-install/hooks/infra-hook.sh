#!/usr/bin/env bash
#
# Reference InfraReady / PostProvision / PreDestroy hook for
# `platform: external` with CAPOCI.
#
# ############################################################################
# # STATUS: NEVER EXECUTED. Written 2026-09-30 with no OCI account available. #
# # Every `oci` CLI invocation below is from the published command reference, #
# # not from a run. The aws-capa sibling of this file was developed against a #
# # live account over several installs; this one was not. Expect flag names   #
# # and JSON shapes to need correction on first contact.                      #
# ############################################################################
#
#
# WHY THIS EXISTS
#
# On an integrated platform the installer creates the cluster's DNS itself
# after CAPI reports the network ready -- for AWS that is
# pkg/infrastructure/aws/clusterapi/aws.go:112. `platform: external` has no
# such code and must not grow any: the point is that the installer knows
# nothing about the provider's cloud. But the records still have to exist,
# because a control-plane machine's pointer ignition aims at
# https://api-int.<clusterDomain>:22623/config/master.
#
# So the seam is here. The installer calls a program the user supplies, at the
# same point an integrated provider would run its own InfraReady, and hands it
# everything it can describe without knowing the provider's API.
#
#
# WHAT THIS HOOK DOES THAT THE AWS ONE DOES NOT
#
# Three things, and the second is the one with no AWS equivalent at all.
#
# 1. DNS, in OCI DNS rather than Route 53. Same job.
#
# 2. ADDS A 22623 LISTENER TO THE API LOAD BALANCER. CAPOCI's network load
#    balancer has exactly one listener, on 6443: every port in
#    cloud/scope/network_load_balancer_reconciler.go is APIServerPort()
#    (:55, :72, :187). There is no equivalent of CAPA's
#    controlPlaneLoadBalancer.additionalListeners. Masters fetch their real
#    config from api-int:22623, so without this they boot with no OS config
#    and the install dies after the full bootstrap timeout.
#
#    This is safe from CAPOCI's reconcile loop, which was checked rather than
#    assumed: IsNLBEqual compares only the display name and the health checker
#    of the single backend set named apiserver-lb-backendset (:301-320), and
#    UpdateNLB sends only DisplayName plus that health checker (:122-150).
#    Neither enumerates listeners, so one added here is not pruned.
#
#    The BACKENDS are a separate problem. CAPOCI registers control-plane
#    machines into apiserver-lb-backendset only (cloud/scope/machine.go:874),
#    and at infra-ready no machine exists yet. So the listener and an empty
#    backend set are created here, and post-provision populates them.
#
# 3. WRITES THE OCI CCM AND CSI CONFIG SECRETS. These need the VCN,
#    service-lb subnet and security list OCIDs, none of which exist before
#    CAPI builds the network -- so they cannot be day-0 manifests. See
#    ../extra-manifests/README.md.
#
#
# THE CONTRACT
#
# The installer execs this program directly -- no shell -- with the
# environment below and whatever arguments install-config gave it. Working
# directory is the install directory.
#
# Exit 0 is success; any other exit aborts the install, and the last few lines
# written are quoted in the installer's error, so the last thing said before
# exiting should be the reason.
#
#   OPENSHIFT_INSTALL_HOOK            "infra-ready", "post-provision" or
#                                     "pre-destroy"
#   OPENSHIFT_INSTALL_INFRA_ID        e.g. mrb-oci1-knzjv
#   OPENSHIFT_INSTALL_CLUSTER_NAME
#   OPENSHIFT_INSTALL_BASE_DOMAIN
#   OPENSHIFT_INSTALL_CLUSTER_DOMAIN  <cluster name>.<base domain>
#   OPENSHIFT_INSTALL_PUBLISH         External | Internal
#   OPENSHIFT_INSTALL_DIR             the install directory
#   OPENSHIFT_INSTALL_MANIFEST_DIR    <install dir>/external-install
#   OPENSHIFT_INSTALL_STATE_DIR       writable; what infra-ready records here
#                                     is what pre-destroy reads
#   OPENSHIFT_INSTALL_CLUSTER_JSON    the core CAPI Cluster object, as JSON
#   OPENSHIFT_INSTALL_INFRA_JSON      the OCICluster, as JSON, verbatim from
#                                     the local control plane
#   OPENSHIFT_INSTALL_KUBECONFIG      the installed cluster's admin kubeconfig.
#                                     Set for every hook and usable by none
#                                     until the cluster's API is serving.
#
# OPENSHIFT_INSTALL_INFRA_JSON is the whole mechanism. The installer cannot
# read .spec.networkSpec.vcn.id off an OCICluster, because it has no OCICluster
# type and must not acquire one. It can copy the object out of its local
# control plane as JSON and let the hook dig. Every path read below is
# CAPOCI's schema, not the installer's.
#
# Note those are SPEC paths, not status. CAPOCI writes the OCIDs of what it
# creates back into the spec -- cloud/scope/vcn_reconciler.go:43,51 for the VCN
# and cloud/scope/subnet_reconciler.go for subnets. OCIClusterStatus has no
# network block at all (api/v1beta2/ocicluster_types.go:85-95).
#
# OPENSHIFT_INSTALL_STATE_DIR is how destroy stays honest. Anything created
# here is outside CAPI's ownership, so nothing else will ever clean it up. The
# rule on this pilot is that each new resource ships with its teardown in the
# same change, and the state file links the two directions.
#
#
# REQUIREMENTS
#
#   oci   the OCI CLI, configured. Uses the same tenancy/user/key as the
#         OCIClusterIdentity Secret, but reads them from ~/.oci/config -- this
#         hook runs on the installer host, not in the cluster.
#   jq
#   oc    only for post-provision and pre-destroy.
#
# The hook never prints a credential, an OCID of a Secret, or the contents of
# any file it writes. Pre-authenticated request URLs in particular are
# credentials: see ../../docs/ignition-delivery.md.

set -euo pipefail

log() { printf '%s %s\n' "[$(date -u +%H:%M:%S)] oci-hook:" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

for tool in oci jq; do
	command -v "${tool}" >/dev/null 2>&1 || die "${tool} is not on PATH"
done

: "${OPENSHIFT_INSTALL_HOOK:?not set -- this program is run by the installer}"
: "${OPENSHIFT_INSTALL_INFRA_ID:?}"
: "${OPENSHIFT_INSTALL_CLUSTER_DOMAIN:?}"
: "${OPENSHIFT_INSTALL_BASE_DOMAIN:?}"
: "${OPENSHIFT_INSTALL_STATE_DIR:?}"

INFRA_ID=${OPENSHIFT_INSTALL_INFRA_ID}
CLUSTER_DOMAIN=${OPENSHIFT_INSTALL_CLUSTER_DOMAIN}
BASE_DOMAIN=${OPENSHIFT_INSTALL_BASE_DOMAIN}
PUBLISH=${OPENSHIFT_INSTALL_PUBLISH:-External}

STATE="${OPENSHIFT_INSTALL_STATE_DIR}/oci-hook-state.json"
APPS_STATE="${OPENSHIFT_INSTALL_STATE_DIR}/oci-hook-apps.json"
CCM_STATE="${OPENSHIFT_INSTALL_STATE_DIR}/oci-ccm-secrets.yaml"

# Written by ../../scripts/run-create-command.sh BEFORE the installer starts,
# recording the bootstrap ignition object and the pre-authenticated request
# over it. The hook cannot discover these -- the offload happens between
# `create ignition-configs` and `create cluster`, with no hook in between --
# so the script hands them over through a file at a path both sides agree on.
# It holds the PAR's ID, not its URL: the ID is an identifier, the URL is a
# credential.
IGNITION_STATE="${OPENSHIFT_INSTALL_DIR:-.}/.external-hook-state/bootstrap-ignition.json"

# The second listener. Named, not numbered, because OCI addresses listeners
# and backend sets by name and pre-destroy has to find them again.
MCS_PORT=22623
MCS_LISTENER="mcs-listener"
MCS_BACKENDSET="mcs-backendset"

# ---------------------------------------------------------------------------
# Reading the infrastructure object
# ---------------------------------------------------------------------------

infra_json() {
	[[ -n ${OPENSHIFT_INSTALL_INFRA_JSON:-} ]] ||
		die "OPENSHIFT_INSTALL_INFRA_JSON is unset -- the installer sets it only when exactly one infrastructure object exists"
	printf '%s' "${OPENSHIFT_INSTALL_INFRA_JSON}"
}

# jq against the OCICluster, failing loudly on an empty result rather than
# letting an empty string flow into an OCI CLI call.
infra_get() {
	local path=$1 desc=$2 value
	value=$(infra_json | jq -r "${path} // empty")
	[[ -n ${value} ]] || die "${desc} not found at ${path} in the OCICluster"
	printf '%s' "${value}"
}

vcn_id()        { infra_get '.spec.networkSpec.vcn.id' 'VCN OCID'; }
compartment()   { infra_get '.spec.compartmentId' 'compartment OCID'; }
nlb_id()        { infra_get '.spec.networkSpec.apiServerLoadBalancer.loadBalancerId' 'API server NLB OCID'; }
api_endpoint()  { infra_get '.spec.controlPlaneEndpoint.host' 'control plane endpoint host'; }

subnet_by_role() {
	infra_get ".spec.networkSpec.vcn.subnets[] | select(.role==\"$1\") | .id" "$1 subnet OCID"
}

seclist_by_role() {
	infra_get ".spec.networkSpec.vcn.subnets[] | select(.role==\"$1\") | .securityList.id" "$1 security list OCID"
}

# ---------------------------------------------------------------------------
# DNS
#
# OCI DNS rather than Route 53, and the shape differs in one way worth
# knowing: OCI has no alias-record concept, so api/api-int are A records
# pointing at the NLB's IP address rather than aliases pointing at the load
# balancer itself. CAPOCI has already resolved that IP for us -- it sets
# .spec.controlPlaneEndpoint.host from the NLB's IP at
# cloud/scope/network_load_balancer_reconciler.go (SetControlPlaneEndpoint).
# ---------------------------------------------------------------------------

create_private_zone() {
	local compartment_id=$1 vcn=$2 existing

	existing=$(oci dns zone list --compartment-id "${compartment_id}" \
		--name "${CLUSTER_DOMAIN}" --scope PRIVATE \
		--query 'data[0].id' --raw-output 2>/dev/null || true)
	if [[ -n ${existing} && ${existing} != "null" ]]; then
		log "private zone ${CLUSTER_DOMAIN} already exists"
		printf '%s' "${existing}"
		return 0
	fi

	# A private zone needs a view, and each VCN has a default one. Looking it
	# up by VCN rather than by name because the default view's name is not
	# documented as stable.
	local view_id
	view_id=$(oci dns resolver list --compartment-id "${compartment_id}" \
		--scope PRIVATE \
		--query "data[?\"attached-vcn-id\"=='${vcn}'] | [0].\"default-view-id\"" \
		--raw-output)
	[[ -n ${view_id} && ${view_id} != "null" ]] ||
		die "no default private DNS view for VCN ${vcn}"

	log "creating private zone ${CLUSTER_DOMAIN}"
	oci dns zone create --compartment-id "${compartment_id}" \
		--name "${CLUSTER_DOMAIN}" --zone-type PRIMARY --scope PRIVATE \
		--view-id "${view_id}" \
		--query 'data.id' --raw-output
}

upsert_a_record() {
	local zone_id=$1 fqdn=$2 ip=$3 scope=$4
	log "upserting A ${fqdn} -> ${ip}"
	oci dns record rrset update --zone-name-or-id "${zone_id}" \
		--domain "${fqdn}" --rtype A --scope "${scope}" \
		--items "$(jq -n --arg d "${fqdn}" --arg r "${ip}" \
			'[{domain:$d, rtype:"A", ttl:60, rdata:$r}]')" \
		--force >/dev/null
}

delete_rrset() {
	local zone_id=$1 fqdn=$2 rtype=$3 scope=$4
	if oci dns record rrset delete --zone-name-or-id "${zone_id}" \
		--domain "${fqdn}" --rtype "${rtype}" --scope "${scope}" \
		--force >/dev/null 2>&1; then
		log "deleted ${rtype} ${fqdn}"
	else
		log "${rtype} ${fqdn} already gone"
	fi
}

# ---------------------------------------------------------------------------
# The 22623 listener
# ---------------------------------------------------------------------------

add_mcs_listener() {
	local nlb=$1

	if oci network-load-balancer backend-set get --network-load-balancer-id "${nlb}" \
		--backend-set-name "${MCS_BACKENDSET}" >/dev/null 2>&1; then
		log "${MCS_BACKENDSET} already exists"
	else
		log "creating backend set ${MCS_BACKENDSET} for port ${MCS_PORT}"
		# Created EMPTY. No machine exists yet at infra-ready; post-provision
		# fills it. SOURCE_IP_ADDRESS rather than round robin because the MCS
		# is stateless but the health check is per-backend and a flapping
		# backend should not move every client.
		oci network-load-balancer backend-set create \
			--network-load-balancer-id "${nlb}" \
			--name "${MCS_BACKENDSET}" \
			--policy FIVE_TUPLE \
			--is-preserve-source false \
			--health-checker "$(jq -n --argjson p "${MCS_PORT}" \
				'{protocol:"HTTPS", port:$p, urlPath:"/healthz",
				  returnCode:200, intervalInMillis:10000,
				  timeoutInMillis:3000, retries:3}')" \
			--wait-for-state SUCCEEDED >/dev/null
	fi

	if oci network-load-balancer listener get --network-load-balancer-id "${nlb}" \
		--listener-name "${MCS_LISTENER}" >/dev/null 2>&1; then
		log "${MCS_LISTENER} already exists"
	else
		log "creating listener ${MCS_LISTENER} on port ${MCS_PORT}"
		oci network-load-balancer listener create \
			--network-load-balancer-id "${nlb}" \
			--name "${MCS_LISTENER}" \
			--default-backend-set-name "${MCS_BACKENDSET}" \
			--port "${MCS_PORT}" --protocol TCP \
			--wait-for-state SUCCEEDED >/dev/null
	fi
}

# ---------------------------------------------------------------------------
# The CCM and CSI config Secrets
#
# Four values, no credentials: useInstancePrincipals means the in-cluster
# components authenticate as the instance. Shapes copied from
# oci-openshift/custom_manifests/manifests/01-oci-driver-configs.yml.
# ---------------------------------------------------------------------------

render_ccm_secrets() {
	local compartment_id=$1 vcn=$2 lb_subnet=$3 lb_seclist=$4

	local cloud_config
	cloud_config=$(cat <<-EOF
	auth:
	  useInstancePrincipals: true
	compartment: ${compartment_id}
	vcn: ${vcn}
	loadBalancer:
	  subnet1: ${lb_subnet}
	  securityListManagementMode: Frontend
	  securityLists:
	    ${lb_subnet}: ${lb_seclist}
	rateLimiter:
	  rateLimitQPSRead: 20.0
	  rateLimitBucketRead: 5
	  rateLimitQPSWrite: 20.0
	  rateLimitBucketWrite: 5
	EOF
	)

	# Both Secrets carry the same document under different keys in different
	# namespaces, which is how Oracle ships it.
	cat > "${CCM_STATE}" <<-EOF
	---
	apiVersion: v1
	kind: Secret
	metadata:
	  name: oci-cloud-controller-manager
	  namespace: oci-cloud-controller-manager
	stringData:
	  cloud-provider.yaml: |
	$(printf '%s\n' "${cloud_config}" | sed 's/^/    /')
	---
	apiVersion: v1
	kind: Secret
	metadata:
	  name: oci-volume-provisioner
	  namespace: oci-csi
	stringData:
	  config.yaml: |
	$(printf '%s\n' "${cloud_config}" | sed 's/^/    /')
	EOF

	log "rendered CCM/CSI config to the state dir ($(wc -c <"${CCM_STATE}") bytes)"
}

# Try to land the Secrets as day-0 content. Whether <install-dir>/openshift
# still exists and is still consumed at infra-ready is UNVERIFIED -- see
# ../extra-manifests/README.md. If it does not, post-provision applies them
# instead, at the cost of the CCM crash-looping until then.
place_ccm_secrets() {
	local openshift_dir="${OPENSHIFT_INSTALL_DIR:-.}/openshift"
	if [[ -d ${openshift_dir} ]]; then
		cp "${CCM_STATE}" "${openshift_dir}/99_external-01-oci-ccm-config.yaml"
		log "placed CCM/CSI config in ${openshift_dir} as day-0 content"
		printf 'day0'
	else
		log "no ${openshift_dir}; CCM/CSI config will be applied at post-provision"
		printf 'post-provision'
	fi
}

# ---------------------------------------------------------------------------
# infra-ready
# ---------------------------------------------------------------------------

infra_ready() {
	local compartment_id vcn nlb api_ip lb_subnet lb_seclist zone_id ccm_mode

	compartment_id=$(compartment)
	vcn=$(vcn_id)
	nlb=$(nlb_id)
	api_ip=$(api_endpoint)
	lb_subnet=$(subnet_by_role service-lb)
	lb_seclist=$(seclist_by_role service-lb)

	log "VCN ${vcn}, NLB ${nlb}, API endpoint ${api_ip}"

	# 1. The 22623 listener, before DNS -- if this fails there is no point
	#    publishing a name that cannot serve the machine config server.
	add_mcs_listener "${nlb}"

	# 2. Private DNS. api-int is what the pointer ignition and the bootstrap
	#    kubeconfig resolve; api is the same address from inside the VCN.
	zone_id=$(create_private_zone "${compartment_id}" "${vcn}")
	upsert_a_record "${zone_id}" "api-int.${CLUSTER_DOMAIN}" "${api_ip}" PRIVATE
	upsert_a_record "${zone_id}" "api.${CLUSTER_DOMAIN}" "${api_ip}" PRIVATE

	# 3. Public api, when the cluster is published externally. The public zone
	#    for the base domain is assumed to exist and to be managed by OCI DNS;
	#    a partner using another DNS provider replaces this one call.
	local public_zone=""
	if [[ ${PUBLISH} == "External" ]]; then
		public_zone=$(oci dns zone list --compartment-id "${compartment_id}" \
			--name "${BASE_DOMAIN}" --scope GLOBAL \
			--query 'data[0].id' --raw-output 2>/dev/null || true)
		if [[ -n ${public_zone} && ${public_zone} != "null" ]]; then
			upsert_a_record "${public_zone}" "api.${CLUSTER_DOMAIN}" "${api_ip}" GLOBAL
		else
			public_zone=""
			log "no public zone for ${BASE_DOMAIN}; skipping the public api record"
		fi
	fi

	# 4. The CCM and CSI config.
	render_ccm_secrets "${compartment_id}" "${vcn}" "${lb_subnet}" "${lb_seclist}"
	ccm_mode=$(place_ccm_secrets)

	# 5. Record everything created, for pre-destroy. Nothing here is owned by
	#    Cluster API, so nothing else will remove it.
	jq -n \
		--arg zone "${zone_id}" \
		--arg public_zone "${public_zone}" \
		--arg nlb "${nlb}" \
		--arg compartment "${compartment_id}" \
		--arg vcn "${vcn}" \
		--arg ccm_mode "${ccm_mode}" \
		'{privateZoneId:$zone, publicZoneId:$public_zone, nlbId:$nlb,
		  compartmentId:$compartment, vcnId:$vcn, ccmMode:$ccm_mode}' >"${STATE}"

	log "infra-ready complete"
}

# ---------------------------------------------------------------------------
# post-provision
# ---------------------------------------------------------------------------

INPUT_SERVICE=""
TIMEOUT_SECONDS=1800

parse_post_provision_args() {
	while [[ $# -gt 0 ]]; do
		case $1 in
		--input-service=*) INPUT_SERVICE=${1#*=} ;;
		--input-service) INPUT_SERVICE=${2:?--input-service needs a value}; shift ;;
		--timeout-seconds=*) TIMEOUT_SECONDS=${1#*=} ;;
		--timeout-seconds) TIMEOUT_SECONDS=${2:?--timeout-seconds needs a value}; shift ;;
		*) die "unknown argument $1" ;;
		esac
		shift
	done
	[[ -n ${INPUT_SERVICE} ]] || die "--input-service is required for post-provision"
}

kube() {
	oc --kubeconfig "${OPENSHIFT_INSTALL_KUBECONFIG:?}" "$@"
}

# Add every control-plane instance to the 22623 backend set.
#
# CAPOCI does this itself for apiserver-lb-backendset
# (cloud/scope/machine.go:874-895) and only for that one, so the second
# backend set is ours to populate.
#
# The bootstrap machine serves 22623 too, and arguably needs to be a backend
# EARLIER than this -- masters fetch their config on first boot, which is
# before post-provision runs. This is the least-settled part of the design and
# is called out in ../../docs/capi-requirements.md.
populate_mcs_backends() {
	local nlb=$1 compartment_id=$2 instances ocid ip count=0

	instances=$(oci compute instance list --compartment-id "${compartment_id}" \
		--lifecycle-state RUNNING \
		--query "data[?\"freeform-tags\".\"openshift-cluster-id\"=='${INFRA_ID}' &&
		          (\"freeform-tags\".\"openshift-machine-role\"=='master' ||
		           \"freeform-tags\".\"openshift-machine-role\"=='bootstrap')].id" \
		--raw-output | jq -r '.[]? // empty')

	[[ -n ${instances} ]] || { log "no control-plane instances tagged for ${INFRA_ID} yet"; return 0; }

	while read -r ocid; do
		[[ -n ${ocid} ]] || continue
		ip=$(oci compute instance list-vnics --instance-id "${ocid}" \
			--query 'data[0]."private-ip"' --raw-output)
		[[ -n ${ip} && ${ip} != "null" ]] || { log "no private IP for ${ocid}; skipping"; continue; }

		if oci network-load-balancer backend create \
			--network-load-balancer-id "${nlb}" \
			--backend-set-name "${MCS_BACKENDSET}" \
			--ip-address "${ip}" --port "${MCS_PORT}" \
			--wait-for-state SUCCEEDED >/dev/null 2>&1; then
			log "added ${ip}:${MCS_PORT} to ${MCS_BACKENDSET}"
			count=$((count + 1))
		else
			log "${ip}:${MCS_PORT} already a backend, or could not be added"
		fi
	done <<<"${instances}"

	log "${count} backend(s) added to ${MCS_BACKENDSET}"
}

# Wait for the ingress Service to get a load balancer address. That load
# balancer is built by the cluster's OWN cloud controller manager, minutes
# into bootstrap, which is why the *.apps wildcard cannot be created at
# infra-ready: at that point nothing has built it and nothing will.
service_address() {
	local ns=${INPUT_SERVICE%%/*} name=${INPUT_SERVICE##*/}
	local deadline=$(( $(date +%s) + TIMEOUT_SECONDS )) addr=""

	log "waiting up to ${TIMEOUT_SECONDS}s for ${ns}/${name} to get an address"
	while [[ $(date +%s) -lt ${deadline} ]]; do
		addr=$(kube -n "${ns}" get service "${name}" \
			-o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
		[[ -n ${addr} ]] && { printf '%s' "${addr}"; return 0; }
		sleep 15
	done
	die "${ns}/${name} had no load balancer address after ${TIMEOUT_SECONDS}s"
}

post_provision() {
	parse_post_provision_args "$@"

	[[ -f ${STATE} ]] || die "no ${STATE}; infra-ready did not run or did not complete"

	local nlb compartment_id zone_id public_zone ccm_mode apps_ip
	nlb=$(jq -r '.nlbId' "${STATE}")
	compartment_id=$(jq -r '.compartmentId' "${STATE}")
	zone_id=$(jq -r '.privateZoneId' "${STATE}")
	public_zone=$(jq -r '.publicZoneId // empty' "${STATE}")
	ccm_mode=$(jq -r '.ccmMode' "${STATE}")

	# 1. Backends for 22623, first -- everything downstream needs the machine
	#    config server reachable.
	populate_mcs_backends "${nlb}" "${compartment_id}"

	# 2. The CCM config, if it could not be day-0 content.
	if [[ ${ccm_mode} != "day0" ]]; then
		log "applying CCM/CSI config"
		kube apply -f "${CCM_STATE}" >/dev/null
	fi

	# 3. The ingress wildcard.
	apps_ip=$(service_address)
	log "ingress address ${apps_ip}"
	upsert_a_record "${zone_id}" "*.apps.${CLUSTER_DOMAIN}" "${apps_ip}" PRIVATE
	if [[ -n ${public_zone} ]]; then
		upsert_a_record "${public_zone}" "*.apps.${CLUSTER_DOMAIN}" "${apps_ip}" GLOBAL
	fi

	jq -n --arg ip "${apps_ip}" '{appsAddress:$ip}' >"${APPS_STATE}"
	log "post-provision complete"
}

# ---------------------------------------------------------------------------
# pre-destroy
# ---------------------------------------------------------------------------

# The load balancer the cluster's CCM built for the ingress Service is not a
# Cluster API resource and deleting the Cluster will not touch it. Deleting the
# Service makes the CCM remove its own load balancer, which is both tidier and
# more correct than deleting the load balancer behind the CCM's back. If the
# API is already gone, fall back to deleting by tag.
destroy_ingress() {
	local ns name
	ns=${INPUT_SERVICE%%/*}
	name=${INPUT_SERVICE##*/}

	if [[ -z ${INPUT_SERVICE} ]]; then
		# pre-destroy takes no arguments, so recover the Service from the
		# install-config's post-provision args if they are reachable; failing
		# that, fall through to the tag sweep.
		ns=openshift-ingress
		name=router-external-default
	fi

	if [[ -n ${OPENSHIFT_INSTALL_KUBECONFIG:-} ]] &&
		kube -n "${ns}" get service "${name}" >/dev/null 2>&1; then
		log "deleting service ${ns}/${name} so the CCM removes its load balancer"
		kube -n "${ns}" delete service "${name}" --wait=true --timeout=5m >/dev/null || true
		return 0
	fi

	log "cluster API not reachable; sweeping load balancers tagged ${INFRA_ID}"
	local compartment_id lbs lb
	compartment_id=$(jq -r '.compartmentId // empty' "${STATE}" 2>/dev/null || true)
	[[ -n ${compartment_id} ]] || { log "no compartment recorded; skipping the sweep"; return 0; }

	lbs=$(oci lb load-balancer list --compartment-id "${compartment_id}" \
		--query "data[?contains(\"display-name\", '${INFRA_ID}')].id" \
		--raw-output 2>/dev/null | jq -r '.[]? // empty' || true)
	while read -r lb; do
		[[ -n ${lb} ]] || continue
		log "deleting load balancer ${lb}"
		oci lb load-balancer delete --load-balancer-id "${lb}" --force >/dev/null || true
	done <<<"${lbs}"
}

# The bootstrap ignition object and its pre-authenticated request.
#
# The PAR is an unauthenticated URL over the cluster's day-0 secrets. Its
# expiry is a backstop; this is the cleanup. Never log the URL -- it IS the
# credential. Log the object name only.
destroy_bootstrap_ignition() {
	local bucket object par_id

	if [[ ! -f ${IGNITION_STATE} ]]; then
		log "no ${IGNITION_STATE}; no bootstrap ignition object was offloaded"
		return 0
	fi

	bucket=$(jq -r '.ignitionBucket // empty' "${IGNITION_STATE}")
	object=$(jq -r '.ignitionObject // empty' "${IGNITION_STATE}")
	par_id=$(jq -r '.ignitionParId // empty' "${IGNITION_STATE}")

	[[ -n ${bucket} ]] || { log "no bucket recorded in ${IGNITION_STATE}"; return 0; }

	if [[ -n ${par_id} ]]; then
		log "deleting the pre-authenticated request over ${object}"
		oci os preauth-request delete --bucket-name "${bucket}" \
			--par-id "${par_id}" --force >/dev/null 2>&1 ||
			log "pre-authenticated request already gone"
	fi
	if [[ -n ${object} ]]; then
		log "deleting object ${object} from ${bucket}"
		oci os object delete --bucket-name "${bucket}" \
			--object-name "${object}" --force >/dev/null 2>&1 ||
			log "object already gone"
	fi
}

# The listener and backend set added at infra-ready. Normally moot, because
# CAPOCI deletes the whole NLB when the Cluster goes -- but a destroy re-run
# after a partial failure may find the NLB still there, and the standing rule
# is that whatever this hook creates, it removes.
destroy_mcs_listener() {
	local nlb
	nlb=$(jq -r '.nlbId // empty' "${STATE}" 2>/dev/null || true)
	[[ -n ${nlb} ]] || return 0

	oci network-load-balancer get --network-load-balancer-id "${nlb}" >/dev/null 2>&1 || {
		log "NLB ${nlb} is already gone"
		return 0
	}

	# Listener before backend set: OCI refuses to delete a backend set that a
	# listener still points at.
	if oci network-load-balancer listener delete --network-load-balancer-id "${nlb}" \
		--listener-name "${MCS_LISTENER}" --force >/dev/null 2>&1; then
		log "deleted listener ${MCS_LISTENER}"
	else
		log "listener ${MCS_LISTENER} already gone"
	fi

	if oci network-load-balancer backend-set delete --network-load-balancer-id "${nlb}" \
		--backend-set-name "${MCS_BACKENDSET}" --force >/dev/null 2>&1; then
		log "deleted backend set ${MCS_BACKENDSET}"
	else
		log "backend set ${MCS_BACKENDSET} already gone"
	fi
}

pre_destroy() {
	# The bootstrap ignition object comes first and is unconditional. It is
	# recorded in a different file, written by the create script before the
	# installer ever started, so an install that failed before infra-ready can
	# still have left an unauthenticated URL over the cluster's day-0 secrets
	# sitting in object storage. That is exactly the case where an early
	# return would be worst.
	destroy_bootstrap_ignition

	if [[ ! -f ${STATE} ]]; then
		log "no ${STATE}; infra-ready left nothing else to remove"
		return 0
	fi

	destroy_ingress
	destroy_mcs_listener

	local zone_id public_zone
	zone_id=$(jq -r '.privateZoneId // empty' "${STATE}")
	public_zone=$(jq -r '.publicZoneId // empty' "${STATE}")

	# The public records first, and by exact name. That zone belongs to the
	# base domain and holds other clusters' records; deleting anything not
	# named here would take out somebody else's cluster.
	if [[ -n ${public_zone} ]]; then
		delete_rrset "${public_zone}" "api.${CLUSTER_DOMAIN}" A GLOBAL
		delete_rrset "${public_zone}" "*.apps.${CLUSTER_DOMAIN}" A GLOBAL
	fi

	# Then the private zone, whole. It was created for this cluster and holds
	# nothing else, so it goes rather than being emptied record by record.
	if [[ -n ${zone_id} ]]; then
		if oci dns zone get --zone-name-or-id "${zone_id}" --scope PRIVATE >/dev/null 2>&1; then
			log "deleting private zone ${CLUSTER_DOMAIN}"
			oci dns zone delete --zone-name-or-id "${zone_id}" --scope PRIVATE \
				--force >/dev/null
		else
			log "private zone is already gone"
		fi
	fi

	rm -f "${STATE}" "${APPS_STATE}" "${CCM_STATE}"
	log "pre-destroy complete"
}

# The three halves are one file so that what a hook creates and what removes it
# can be read against each other. Only post-provision takes arguments;
# forwarding "$@" to it and to nothing else is deliberate, so that an argument
# sent to the wrong hook is an error rather than a no-op.
case ${OPENSHIFT_INSTALL_HOOK} in
infra-ready) infra_ready ;;
post-provision) post_provision "$@" ;;
pre-destroy) pre_destroy ;;
*) die "unknown hook ${OPENSHIFT_INSTALL_HOOK}" ;;
esac
