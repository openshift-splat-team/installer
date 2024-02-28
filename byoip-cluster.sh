#!/bin/bash


if [[ -s $1 ]]; then
  echo "You must specify the command"
  exit 1
fi
if [[ -s $2 ]]; then
  echo "You must specify the cluster id"
  exit 1
fi
CMD=$1; shift
CLUSTER_ID=$1

CLUSTER_NAME=byoip-capa-$CLUSTER_ID
INSTALL_DIR=${HOME}/openshift-labs/$CLUSTER_NAME

build() {
  rm -vf ${PWD}/cluster-api/bin/linux_amd64/cluster-api-provider-aws bin/
  cp -v /home/mtulio/go/src/github.com/openshift-splat-team/cluster-api-provider-aws/bin/manager ${PWD}/cluster-api/bin/linux_amd64/cluster-api-provider-aws
  SKIP_TERRAFORM=y OPENSHIFT_INSTALL_CLUSTER_API=1 ./hack/build.sh
}

install() {
  PULL_SECRET_FILE="${HOME}/.openshift/pull-secret-latest.json"

  #CLUSTER_BASE_DOMAIN=origin-ci-int-aws.dev.rhcloud.com
  CLUSTER_BASE_DOMAIN=devcluster.openshift.com
  SSH_PUB_KEY_FILE=$HOME/.ssh/id_rsa.pub
  REGION=us-east-1
  AWS_REGION=$REGION

  PUBLIC_IPV4_POOL_ID="ipv4pool-ec2-09e5e971e86699d07"

  mkdir -p $INSTALL_DIR && cd $INSTALL_DIR


  echo "> Creating install-config.yaml"
  # Create a single-AZ install config
  mkdir -p ${INSTALL_DIR}
  cat <<EOF | envsubst > ${INSTALL_DIR}/install-config.yaml
apiVersion: v1
baseDomain: ${CLUSTER_BASE_DOMAIN}
featureSet: CustomNoUpgrade
featureGates:
- ClusterAPIInstall=true
metadata:
  name: "${CLUSTER_NAME}"
platform:
  aws:
    region: ${REGION}
    publicIpv4Pool: ${PUBLIC_IPV4_POOL_ID}
    ElasticIps:
    - eipalloc-0289332101bdacca1
    ec2:
      elasticIp:
        publicIpv4Pool: ${PUBLIC_IPV4_POOL_ID}
        allocatedIps:
        - eipalloc-0289332101bdacca1
publish: External
pullSecret: '$(cat ${PULL_SECRET_FILE} |awk -v ORS= -v OFS= '{$1=$1}1')'
sshKey: |
  $(cat ${SSH_PUB_KEY_FILE})
EOF

  #OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="quay.io/openshift-release-dev/ocp-release:4.16.0-ec.2-x86_64" \
  #openshift-install-capa create manifests --dir ${INSTALL_DIR} --log-level=debug

  OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="quay.io/openshift-release-dev/ocp-release:4.16.0-ec.2-x86_64" \
  openshift-install-capa create cluster --dir ${INSTALL_DIR} --log-level=debug | tee -a ${INSTALL_DIR}/install.log

}


destroy() {
  openshift-install-capa destroy cluster --dir ${INSTALL_DIR} --log-level=debug | tee -a ${INSTALL_DIR}/install.log
}

case $CMD in
  "create") build; create;;
  "build") build;;
  "destroy") destroy;;
  *) echo "Commmand $CMD not found";;
esac