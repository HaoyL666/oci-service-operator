#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
kind_bin=${KIND:-kind}
kubectl_bin=${KUBECTL:-kubectl}
helm_bin=${HELM:-helm}
docker_bin=${DOCKER:-docker}
group=${GROUP:-psql}
cluster_name=${KIND_CLUSTER_NAME:-"osok-${group}-helm"}
node_image=${KIND_NODE_IMAGE:-kindest/node:v1.29.4@sha256:3abb816a5b1061fb15c6e9e60856ec40d56b7b52bcea5f5f1350bc6e2320b6f8}
namespace=${TEST_NAMESPACE:-"oci-service-operator-${group}-system"}
install_version=${INSTALL_VERSION:-v0.0.0-oke44460}
upgrade_version=${UPGRADE_VERSION:-v0.0.1-oke44460}
image_repository=${CONTROLLER_IMAGE_REPOSITORY:-"osok-${group}-helm-test"}
work_root=${WORK_ROOT:-"${TMPDIR:-/tmp}/osok-${group}-helm-kind"}
release_name="osok-${group}"
manager_name="oci-service-operator-${group}-controller-manager"

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
			GROUP="${group}" \
			VERSION="${version}" \
			CONTROLLER_IMG="${image_repository}:${version}" \
			HELM="${helm_bin}" \
			HELM_OUTPUT_DIR="${output_dir}/charts" \
			HELM_WORK_DIR="${output_dir}/work" >&2
	)
	printf '%s\n' "${output_dir}/charts/oci-service-operator-${group}-chart-${version#v}.tgz"
}

install_chart=$(build_chart "${install_version}")
"${helm_bin}" install "${release_name}" "${install_chart}" \
	--namespace "${namespace}" \
	--set image.pullPolicy=IfNotPresent \
	--wait \
	--timeout 120s

"${kubectl_bin}" -n "${namespace}" get deployment "${manager_name}"
"${kubectl_bin}" -n "${namespace}" rollout status \
	"deployment/${manager_name}" \
	--timeout=120s

upgrade_chart=$(build_chart "${upgrade_version}")
"${helm_bin}" show crds "${upgrade_chart}" | "${kubectl_bin}" apply -f -
"${helm_bin}" upgrade "${release_name}" "${upgrade_chart}" \
	--namespace "${namespace}" \
	--set image.pullPolicy=IfNotPresent \
	--wait \
	--timeout 120s

"${helm_bin}" uninstall "${release_name}" --namespace "${namespace}" --wait
if "${kubectl_bin}" -n "${namespace}" get deployment "${manager_name}" >/dev/null 2>&1; then
	echo "deployment still exists after Helm uninstall" >&2
	exit 1
fi
"${helm_bin}" show crds "${upgrade_chart}" | "${kubectl_bin}" get -f - >/dev/null
"${helm_bin}" show crds "${upgrade_chart}" | "${kubectl_bin}" delete -f -

echo "${group} Helm install, upgrade, and uninstall test passed"
