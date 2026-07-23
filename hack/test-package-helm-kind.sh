#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
kind_bin=${KIND:-kind}
kubectl_bin=${KUBECTL:-kubectl}
helm_bin=${HELM:-helm}
docker_bin=${DOCKER:-docker}
cluster_name=${KIND_CLUSTER_NAME:-osok-psql-helm}
node_image=${KIND_NODE_IMAGE:-kindest/node:v1.29.4@sha256:3abb816a5b1061fb15c6e9e60856ec40d56b7b52bcea5f5f1350bc6e2320b6f8}
namespace=${TEST_NAMESPACE:-oci-service-operator-psql-system}
install_version=${INSTALL_VERSION:-v0.0.0-oke44460}
upgrade_version=${UPGRADE_VERSION:-v0.0.1-oke44460}
image_repository=${CONTROLLER_IMAGE_REPOSITORY:-osok-psql-helm-test}
work_root=${WORK_ROOT:-"${TMPDIR:-/tmp}/osok-psql-helm-kind"}

cleanup() {
	"${kind_bin}" delete cluster --name "${cluster_name}" >/dev/null 2>&1 || true
	"${docker_bin}" image rm \
		"${image_repository}:${install_version}" \
		"${image_repository}:${upgrade_version}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
"${kind_bin}" create cluster --name "${cluster_name}" --image "${node_image}" --wait 120s

node_arch=$("${docker_bin}" image inspect "${node_image}" --format '{{.Architecture}}')
mkdir -p "${work_root}/manager-image"
(
	cd "${ROOT_DIR}"
	CGO_ENABLED=0 GOOS=linux GOARCH="${node_arch}" \
		go build -o "${work_root}/manager-image/manager" ./hack/helm-test-manager
)
"${docker_bin}" build \
	--file "${ROOT_DIR}/hack/helm-test-manager.Dockerfile" \
	--tag "${image_repository}:${install_version}" \
	"${work_root}/manager-image"
"${docker_bin}" tag \
	"${image_repository}:${install_version}" \
	"${image_repository}:${upgrade_version}"
"${kind_bin}" load docker-image \
	--name "${cluster_name}" \
	"${image_repository}:${install_version}" \
	"${image_repository}:${upgrade_version}"

"${kubectl_bin}" create namespace "${namespace}"
"${kubectl_bin}" label namespace "${namespace}" \
	pod-security.kubernetes.io/enforce=restricted \
	pod-security.kubernetes.io/enforce-version=latest
"${kubectl_bin}" -n "${namespace}" create secret generic ocicredentials

build_chart() {
	local version=$1
	local output_dir="${work_root}/${version#v}"
	(
		cd "${ROOT_DIR}"
		make package-helm \
			GROUP=psql \
			VERSION="${version}" \
			CONTROLLER_IMG="${image_repository}:${version}" \
			HELM="${helm_bin}" \
			HELM_OUTPUT_DIR="${output_dir}/charts" \
			HELM_WORK_DIR="${output_dir}/work" >&2
	)
	printf '%s\n' "${output_dir}/charts/oci-service-operator-psql-chart-${version#v}.tgz"
}

install_chart=$(build_chart "${install_version}")
"${helm_bin}" install osok-psql "${install_chart}" \
	--namespace "${namespace}" \
	--set image.pullPolicy=IfNotPresent \
	--wait \
	--timeout 120s

"${kubectl_bin}" -n "${namespace}" get deployment oci-service-operator-psql-controller-manager
"${kubectl_bin}" -n "${namespace}" rollout status \
	deployment/oci-service-operator-psql-controller-manager \
	--timeout=120s
if ! "${kubectl_bin}" auth can-i get secrets \
	--as "system:serviceaccount:${namespace}:oci-service-operator-psql-controller-manager" \
	--namespace "${namespace}"; then
	echo "controller service account cannot get referenced Secrets" >&2
	exit 1
fi
if "${kubectl_bin}" auth can-i list secrets \
	--as "system:serviceaccount:${namespace}:oci-service-operator-psql-controller-manager" \
	--namespace "${namespace}"; then
	echo "controller service account can unexpectedly list Secrets" >&2
	exit 1
fi

upgrade_chart=$(build_chart "${upgrade_version}")
"${helm_bin}" show crds "${upgrade_chart}" | "${kubectl_bin}" apply -f -
"${helm_bin}" upgrade osok-psql "${upgrade_chart}" \
	--namespace "${namespace}" \
	--set image.pullPolicy=IfNotPresent \
	--wait \
	--timeout 120s

"${helm_bin}" uninstall osok-psql --namespace "${namespace}" --wait
if "${kubectl_bin}" -n "${namespace}" get deployment oci-service-operator-psql-controller-manager >/dev/null 2>&1; then
	echo "deployment still exists after Helm uninstall" >&2
	exit 1
fi
"${kubectl_bin}" get crd dbsystems.psql.oracle.com >/dev/null
"${kubectl_bin}" delete crd dbsystems.psql.oracle.com

echo "PostgreSQL Helm install, upgrade, and uninstall test passed"
