#!/usr/bin/env bash

#
# Drives a `platform: external` install on OCI with CAPOCI.
#
# ############################################################################
# # STATUS: NEVER EXECUTED. Written 2026-09-30 with no OCI account available. #
# # The aws-capa counterpart of this script was developed over a dozen runs;  #
# # this one has made zero. Treat it as an executable description of the      #
# # workflow rather than as working automation.                               #
# ############################################################################
#
#
# THREE PHASES, NOT TWO
#
# The AWS version of this script runs `create manifests`, substitutes the
# infrastructure ID, then `create cluster`. Two phases are enough there because
# CAPA uploads the oversized bootstrap ignition to S3 itself.
#
# CAPOCI cannot: it has no object-store offload of any kind -- no
# object-storage field in api/v1beta2/, no objectstorage client under
# cloud/services/ -- and OCI caps all instance metadata at 32,000 bytes, which
# the bootstrap ignition exceeds by two orders of magnitude.
#
# So a third phase sits between `create ignition-configs` and `create cluster`:
# upload bootstrap.ign, mint a pre-authenticated request over it, and replace
# the file on disk with a pointer config. The installer reloads it, because
# bootstrap.Bootstrap implements Load()
# (pkg/asset/ignition/bootstrap/bootstrap.go:39-41) and is in the
# IgnitionConfigs target (pkg/asset/targets/targets.go:56-64). No installer
# change -- see ../docs/ignition-delivery.md.
#
#   phase 1  create manifests        -> the infrastructure ID exists
#   phase 2  create ignition-configs -> bootstrap.ign exists, oversized
#   phase 3  create cluster          -> reads the stub this script wrote
#
#
# WHAT YOU NEED BEFORE THIS WILL RUN
#
#   oci, jq, python3 on PATH; ~/.oci/config configured
#   OCI_COMPARTMENT_ID       target compartment OCID
#   OCI_REGION               e.g. us-ashburn-1
#   OCI_IMAGE_ID             the RHCOS custom image OCID. NOTHING PROVIDES THIS
#                            YET -- see ../docs/boot-image.md. This is the one
#                            prerequisite that is not merely unconfigured but
#                            unsolved.
#   OCI_IGNITION_BUCKET      a private bucket for the bootstrap ignition
#   OCI_CREDENTIALS_FILE     a YAML Secret for CAPOCI's OCIClusterIdentity,
#                            outside this workspace. Never read by this script
#                            beyond handing its path to `oc apply`.
#   PULL_SECRET_FILE         default ~/.oci/pull-secret.json
#   CAPOCI_ARTIFACTS         default /tmp/capoci-artifacts, holding
#                            capoci-shim.sh, the cluster-api-provider-oci
#                            binary and oci-infrastructure-components.yaml
#
# Usage: run-create-command.sh <version> [infra-only]
#

set -eux

CLUSTER_VERSION=${1:-0}

if [[ ${CLUSTER_VERSION} -eq 0 ]]; then
	echo "A valid version must be provided. Got: ${CLUSTER_VERSION}"
	exit 1
fi

CLUSTER_NAME=mrb-oci${CLUSTER_VERSION}
INSTALL_DIR=install-dir-${CLUSTER_NAME}

: "${OCI_COMPARTMENT_ID:?set it to the target compartment OCID}"
: "${OCI_REGION:?set it to the target region, e.g. us-ashburn-1}"
: "${OCI_IGNITION_BUCKET:?set it to a private object storage bucket}"
: "${OCI_CREDENTIALS_FILE:?set it to the OCIClusterIdentity Secret manifest}"

# Checked separately so the failure says what is actually wrong. There is no
# RHCOS image for OCI today and this variable has nothing to point at until
# someone imports one.
if [[ -z ${OCI_IMAGE_ID:-} ]]; then
	echo "OCI_IMAGE_ID is unset. There is no published RHCOS artifact for OCI;"
	echo "read ../docs/boot-image.md and import one before running this."
	exit 1
fi

CAPOCI_ARTIFACTS=${CAPOCI_ARTIFACTS:-/tmp/capoci-artifacts}
for f in capoci-shim.sh cluster-api-provider-oci oci-infrastructure-components.yaml; do
	if [[ ! -e ${CAPOCI_ARTIFACTS}/${f} ]]; then
		echo "missing ${CAPOCI_ARTIFACTS}/${f}; see ./README.md for how to produce the artifacts"
		exit 1
	fi
done

for tool in oci jq python3; do
	command -v "${tool}" >/dev/null 2>&1 || { echo "${tool} is not on PATH"; exit 1; }
done

# Must fail if the directory exists: a second run under the same version would
# install over the first run's state.
mkdir "${INSTALL_DIR}"

cp -v install-config.yaml "${INSTALL_DIR}/"
cp -rvf external-install/ "${INSTALL_DIR}/external-install/"

# CAPOCI's OCI API key, for the local control plane only.
#
# OCICluster.spec.identityRef points at an OCIClusterIdentity, which points at
# a Secret holding tenancy/user/fingerprint/key/region
# (cloud/util/util.go:118-163). The identity object is in
# external-install/cluster.yaml, in the clear, because it holds no secret. The
# Secret itself lives outside this workspace and is copied in by path, with no
# command printing its content.
#
# IT NEVER REACHES THE INSTALLED CLUSTER. The installer applies
# external-install/ to its TEMPORARY LOCAL control plane -- envtest etcd plus
# kube-apiserver on this host -- which is torn down when the install finishes.
# Nothing copies Secrets from there to the target. This matters because the
# in-cluster OCI CCM and CSI use instance principals and need no API key at
# all; see external-install/extra-manifests/README.md.
if [[ ! -s ${OCI_CREDENTIALS_FILE} ]]; then
	echo "no OCIClusterIdentity Secret at ${OCI_CREDENTIALS_FILE}"
	exit 1
fi
cp "${OCI_CREDENTIALS_FILE}" "${INSTALL_DIR}/external-install/00_oci-credentials.yaml"
chmod 0600 "${INSTALL_DIR}/external-install/00_oci-credentials.yaml"

sed -i "s/^  name: CHANGE-ME$/  name: ${CLUSTER_NAME}/" "${INSTALL_DIR}/install-config.yaml"
sed -i "s/^baseDomain: CHANGE-ME$/baseDomain: ${BASE_DOMAIN:?set it to the DNS zone}/" \
	"${INSTALL_DIR}/install-config.yaml"

# Assert the substitution landed. On the AWS pilot a template placeholder
# drifted, every sed silently matched nothing, and three clusters in a row were
# named after the template instead of the version argument -- visible only as a
# wrong infrastructure ID, long after resources existed.
if [[ $(grep -c "^  name: ${CLUSTER_NAME}$" "${INSTALL_DIR}/install-config.yaml") -ne 1 ]]; then
	echo "install-config metadata.name is not ${CLUSTER_NAME}; the template placeholder has drifted"
	exit 1
fi
if grep -q 'CHANGE-ME' "${INSTALL_DIR}/install-config.yaml"; then
	echo "a CHANGE-ME placeholder survived in install-config.yaml"
	exit 1
fi

# The template carries `pullSecret: ""` and the real value lives outside this
# workspace, so the file a human or an agent may read and edit never holds a
# credential. Only this per-run copy does, and it is written by a command that
# emits nothing: the secret is passed BY PATH, never interpolated into a
# command line, so the `set -x` above cannot trace it either.
PULL_SECRET_FILE=${PULL_SECRET_FILE:-${HOME}/.oci/pull-secret.json}

if [[ ! -s ${PULL_SECRET_FILE} ]]; then
	echo "no pull secret at ${PULL_SECRET_FILE}; set PULL_SECRET_FILE to its location"
	exit 1
fi

python3 - "${INSTALL_DIR}/install-config.yaml" "${PULL_SECRET_FILE}" <<'PY'
import json, pathlib, sys
cfg, secret = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
text = cfg.read_text()
if 'pullSecret: ""' not in text:
    sys.exit("install-config has no pullSecret placeholder to fill")
# json.dumps emits a correctly escaped double-quoted scalar, valid YAML.
cfg.write_text(text.replace(
    'pullSecret: ""', "pullSecret: " + json.dumps(secret.read_text().strip()), 1))
PY

# Verify by the SHAPE of what should no longer be there, never by confirming
# the placeholder is present: an edit that lands fully and one that half-lands
# look identical from the placeholder's side.
if [[ $(grep -c '^pullSecret: ""$' "${INSTALL_DIR}/install-config.yaml") -ne 0 ]]; then
	echo "pull secret placeholder survived injection"
	exit 1
fi

export OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="${RELEASE_IMAGE:-quay.io/openshift-release-dev/ocp-release:5.1.0-ec.1-x86_64}"

# A MODE=dev build reads its data assets from disk rather than from the
# embedded copy, and resolves this path relative to the working directory --
# which is this one, not the clone. Without it `create manifests` cannot read
# the CoreOS stream metadata and dies before touching the cloud.
export OPENSHIFT_INSTALL_DATA=../installer/data/data

# Fail here, where the message is about the path, rather than several commands
# later where it is "open data/coreos/coreos-rhel-10.json: no such file or
# directory" and reads like a missing release artifact.
if [[ ! -d ${OPENSHIFT_INSTALL_DATA}/coreos ]]; then
	echo "OPENSHIFT_INSTALL_DATA=${OPENSHIFT_INSTALL_DATA} has no coreos/ -- it must"
	echo "point at the clone's data/data (note the doubled component)."
	exit 1
fi

# Every installer invocation below is ./openshift-install deliberately. A stale
# binary earlier on PATH answers for a different release and says nothing about
# it, which is worse than failing.
if [[ ! -x ./openshift-install ]]; then
	echo "no ./openshift-install here; copy or symlink the build you mean to test"
	exit 1
fi

# ----------------------------------------------------------------- phase one

./openshift-install create manifests --log-level=debug --dir="${INSTALL_DIR}"

# Infrastructure.status.infrastructureName, from a generated, non-secret file.
# grep -oE emits only the matched line, so nothing else in the manifest is
# printed -- which matters because the manifest tree as a whole is a credential
# store.
INFRA_ID=$(grep -oE '^  infrastructureName: .*' \
	"${INSTALL_DIR}/manifests/cluster-infrastructure-02-config.yml" | awk '{print $2}')

if [[ -z ${INFRA_ID} ]]; then
	echo "could not read infrastructureName from the generated manifests"
	exit 1
fi

CLUSTER_DOMAIN="${CLUSTER_NAME}.${BASE_DOMAIN}"

# Every token in external-install/, in one place. Machine network references
# are subnetName/nsgNames selectors rather than OCIDs
# (api/v1beta2/types.go), so nothing here needs a second pass after CAPOCI
# builds the network.
sed -i \
	-e "s/CLUSTER-ID/${INFRA_ID}/g" \
	-e "s|COMPARTMENT-OCID|${OCI_COMPARTMENT_ID}|g" \
	-e "s|IMAGE-OCID|${OCI_IMAGE_ID}|g" \
	-e "s/REGION/${OCI_REGION}/g" \
	-e "s/CLUSTERDNS/${CLUSTER_DOMAIN}/g" \
	"${INSTALL_DIR}/external-install/cluster.yaml" \
	"${INSTALL_DIR}/external-install/00_oci-credentials.yaml" \
	"${INSTALL_DIR}"/external-install/machines/*.yaml

if grep -rqE 'CLUSTER-ID|COMPARTMENT-OCID|IMAGE-OCID|CLUSTERDNS' \
	"${INSTALL_DIR}/external-install/"; then
	echo "placeholders survived substitution under ${INSTALL_DIR}/external-install/"
	grep -rlE 'CLUSTER-ID|COMPARTMENT-OCID|IMAGE-OCID|CLUSTERDNS' \
		"${INSTALL_DIR}/external-install/"
	exit 1
fi

# ----------------------------------------------------------------- phase two

./openshift-install create ignition-configs --log-level=debug --dir="${INSTALL_DIR}"

BOOTSTRAP_IGN="${INSTALL_DIR}/bootstrap.ign"
[[ -s ${BOOTSTRAP_IGN} ]] || { echo "no ${BOOTSTRAP_IGN} after create ignition-configs"; exit 1; }

# Size and hash only. The file is the cluster's day-0 secret material and is
# never printed, and the hash is the only durable record of which bytes were
# uploaded.
BOOTSTRAP_BYTES=$(wc -c <"${BOOTSTRAP_IGN}")
BOOTSTRAP_SHA=$(sha256sum "${BOOTSTRAP_IGN}" | cut -d' ' -f1)
echo "bootstrap.ign: ${BOOTSTRAP_BYTES} bytes, sha256 ${BOOTSTRAP_SHA}"

# --------------------------------------------------------------- phase two.5
#
# Offload. This is the part with no AWS equivalent.

OBJECT_NAME="${INFRA_ID}/bootstrap.ign"

oci os object put \
	--bucket-name "${OCI_IGNITION_BUCKET}" \
	--name "${OBJECT_NAME}" \
	--file "${BOOTSTRAP_IGN}" \
	--content-type application/json \
	--force >/dev/null

echo "uploaded ${OBJECT_NAME} to ${OCI_IGNITION_BUCKET}"

# The pre-authenticated request. Its URL is an UNAUTHENTICATED credential over
# everything in bootstrap.ign, so:
#   - it is never echoed, which is why the `set -x` is suspended around the
#     commands that touch it;
#   - it expires in two hours, enough for a bootstrap and no more;
#   - hooks/infra-hook.sh deletes it at pre-destroy, and the expiry is only a
#     backstop for when that does not run.
PAR_EXPIRY=$(date -u -d '+2 hours' +%Y-%m-%dT%H:%M:%SZ)

set +x   # nothing below this line may be traced until the URL is out of scope
PAR_JSON=$(oci os preauth-request create \
	--bucket-name "${OCI_IGNITION_BUCKET}" \
	--name "${INFRA_ID}-bootstrap" \
	--object-name "${OBJECT_NAME}" \
	--access-type ObjectRead \
	--time-expires "${PAR_EXPIRY}" 2>/dev/null)

PAR_ID=$(printf '%s' "${PAR_JSON}" | jq -r '.data.id')
PAR_PATH=$(printf '%s' "${PAR_JSON}" | jq -r '.data."access-uri"')
unset PAR_JSON

if [[ -z ${PAR_PATH} || ${PAR_PATH} == "null" ]]; then
	set -x
	echo "could not mint a pre-authenticated request for ${OBJECT_NAME}"
	exit 1
fi

PAR_URL="https://objectstorage.${OCI_REGION}.oraclecloud.com${PAR_PATH}"

# The pointer config. Roughly 300 bytes against OCI's 23,900-byte raw budget,
# so the thing that could not fit now fits with room to spare.
python3 - "${BOOTSTRAP_IGN}" <<PY
import json, pathlib, sys
pathlib.Path(sys.argv[1]).write_text(json.dumps({
    "ignition": {
        "version": "3.2.0",
        "config": {"merge": [{"source": "${PAR_URL}"}]},
    }
}))
PY

unset PAR_URL PAR_PATH
set -x

STUB_BYTES=$(wc -c <"${BOOTSTRAP_IGN}")
echo "replaced bootstrap.ign with a ${STUB_BYTES}-byte pointer config"

if [[ ${STUB_BYTES} -gt 23900 ]]; then
	echo "the pointer config is larger than OCI's raw userdata budget; something is wrong"
	exit 1
fi

# Record what pre-destroy has to clean up. The hook reads this file; writing it
# here rather than in the hook is what keeps the two directions linked, and the
# PAR ID is an identifier, not the URL, so this file is not itself a credential.
mkdir -p "${INSTALL_DIR}/.external-hook-state"
jq -n \
	--arg bucket "${OCI_IGNITION_BUCKET}" \
	--arg object "${OBJECT_NAME}" \
	--arg par "${PAR_ID}" \
	--arg sha "${BOOTSTRAP_SHA}" \
	'{ignitionBucket:$bucket, ignitionObject:$object, ignitionParId:$par, ignitionSha256:$sha}' \
	>"${INSTALL_DIR}/.external-hook-state/bootstrap-ignition.json"

unset PAR_ID

# --------------------------------------------------------------- phase three

if [[ "${2:-}" == "infra-only" ]]; then
	# Stops after the InfraReady hook and before any ignition Secret or machine
	# is created -- which is also the cheapest way to test the hook's DNS,
	# 22623 listener and CCM Secret work without booting anything.
	export OPENSHIFT_INSTALL_INFRASTRUCTURE_ONLY=true
fi

./openshift-install create cluster --log-level=debug --dir="${INSTALL_DIR}"
