#!/usr/bin/env bash

#
# Counterpart to run-create-command.sh: removes what a pilot run created,
# through the provider that created it.
#
# STATUS: NEVER EXECUTED. See run-create-command.sh.
#
# `destroy cluster` restarts the local control plane, reapplies the objects
# recorded under <install-dir>/.clusterapi_output/, runs the preDestroy hook,
# and then deletes the Cluster so CAPOCI tears down what it owns. It needs
# three things the install left behind:
#
#   <install-dir>/metadata.json           the provider artifact paths
#   <install-dir>/.clusterapi_output/     the objects, with their OCIDs
#   the artifacts themselves, still at the paths metadata.json names
#
# WHAT CAPOCI DOES NOT OWN, and therefore what the preDestroy hook has to
# remove instead -- see external-install/hooks/infra-hook.sh:
#
#   the DNS zone and its records            created by infraReady
#   the 22623 listener and backend set      created by infraReady
#   the ingress load balancer               created by the cluster's own CCM
#   the bootstrap ignition object and its   created by run-create-command.sh
#     pre-authenticated request               before the installer ever ran
#
# The last of those is the one to care about if a destroy fails: it is an
# unauthenticated URL over the cluster's day-0 secrets. The pre-authenticated
# request expires in two hours regardless, but check it is gone.
#
# Usage: run-destroy-command.sh <version> [dir-suffix]
#

set -eux

CLUSTER_VERSION=${1:-0}

if [[ ${CLUSTER_VERSION} -eq 0 ]]; then
	echo "A valid version must be provided. Got: ${CLUSTER_VERSION}"
	exit 1
fi

CLUSTER_NAME=mrb-oci${CLUSTER_VERSION}
INSTALL_DIR=install-dir-${CLUSTER_NAME}${2:-}

if [[ ! -d ${INSTALL_DIR} ]]; then
	echo "No such install directory: ${INSTALL_DIR}"
	exit 1
fi

# Fail early and legibly rather than part-way through a teardown.
if [[ ! -f ${INSTALL_DIR}/metadata.json ]]; then
	echo "${INSTALL_DIR}/metadata.json is missing; nothing identifies the cluster to remove"
	exit 1
fi
if ! compgen -G "${INSTALL_DIR}/.clusterapi_output/*.yaml" >/dev/null; then
	echo "${INSTALL_DIR}/.clusterapi_output holds no manifests; nothing identifies the infrastructure to remove"
	exit 1
fi

for tool in oci jq; do
	command -v "${tool}" >/dev/null 2>&1 || { echo "${tool} is not on PATH (the preDestroy hook needs it)"; exit 1; }
done

# The same override run-create-command.sh sets, and for the same reason: a
# MODE=dev build reads its data assets from disk rather than from the binary.
# Destroy needs them too -- it unpacks the envtest control plane and the
# embedded Cluster API components before it can restore anything.
export OPENSHIFT_INSTALL_DATA=../installer/data/data

./openshift-install destroy cluster --log-level=debug --dir="${INSTALL_DIR}"

# Independent check, because a preDestroy hook that silently did nothing and
# one that worked are indistinguishable from the installer's exit code.
IGNITION_STATE="${INSTALL_DIR}/.external-hook-state/bootstrap-ignition.json"
if [[ -f ${IGNITION_STATE} ]]; then
	BUCKET=$(jq -r '.ignitionBucket' "${IGNITION_STATE}")
	OBJECT=$(jq -r '.ignitionObject' "${IGNITION_STATE}")
	if oci os object head --bucket-name "${BUCKET}" --name "${OBJECT}" >/dev/null 2>&1; then
		echo "WARNING: ${OBJECT} is still in ${BUCKET}. It holds the cluster's"
		echo "day-0 secrets. Delete it, and any pre-authenticated request over it:"
		echo "  oci os object delete --bucket-name ${BUCKET} --object-name ${OBJECT} --force"
		exit 1
	fi
	echo "bootstrap ignition object is gone from ${BUCKET}"
fi
