#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

usage() {
	cat <<'EOF'
Usage:
  VERSION=v2.3.0 CONTROLLER_IMG=registry/image:v2.3.0 hack/package-helm.sh build <group>
EOF
}

if [[ $# -ne 2 || $1 != "build" ]]; then
	usage
	exit 1
fi

group=$2
metadata_file="${ROOT_DIR}/packages/${group}/metadata.env"
if [[ ! -f "${metadata_file}" ]]; then
	echo "unknown package group: ${group}" >&2
	exit 1
fi
# shellcheck disable=SC1090
source "${metadata_file}"

: "${VERSION:?VERSION must be set and retain its leading v}"
: "${CONTROLLER_IMG:?CONTROLLER_IMG must be set to an exact tagged or digest-pinned image}"
: "${PACKAGE_NAME:?missing PACKAGE_NAME in ${metadata_file}}"
: "${PACKAGE_NAMESPACE:?missing PACKAGE_NAMESPACE in ${metadata_file}}"

helm_bin=${HELM:-helm}
out_dir=${OUT_DIR:-"${ROOT_DIR}/dist/charts"}
work_dir=${WORK_DIR:-"${out_dir}/.work/${group}"}
chart_name="${PACKAGE_NAME}-chart"
chart_version=${VERSION#v}
chart_dir="${out_dir}/${chart_name}"
package_manifest="${work_dir}/package-install.yaml"
helm_manifest="${work_dir}/helm-install.yaml"
companion_crd="${out_dir}/${group}-crds-${chart_version}.yaml"
archive="${out_dir}/${chart_name}-${chart_version}.tgz"
skeleton="${ROOT_DIR}/charts/oci-service-operator-service-chart"
release_name="osok-${group}"
namespace="${PACKAGE_NAMESPACE}"

mkdir -p "${out_dir}" "${work_dir}"

CONTROLLER_GEN_RUNNER="${CONTROLLER_GEN_RUNNER:-${ROOT_DIR}/hack/with-controller-gen-godebug.sh}" \
CONTROLLER_GEN="${CONTROLLER_GEN:-controller-gen}" \
KUSTOMIZE="${KUSTOMIZE:-kustomize}" \
CONTROLLER_IMG="${CONTROLLER_IMG}" \
OUT="${package_manifest}" \
	"${ROOT_DIR}/hack/package.sh" render "${group}"

(
	cd "${ROOT_DIR}"
	go run ./cmd/packagehelm generate \
		--group "${group}" \
		--version "${VERSION}" \
		--controller-image "${CONTROLLER_IMG}" \
		--manifest "${package_manifest}" \
		--skeleton "${skeleton}" \
		--output-dir "${chart_dir}" \
		--crd-output "${companion_crd}"
)

"${helm_bin}" lint "${chart_dir}" --strict
"${helm_bin}" template "${release_name}" "${chart_dir}" \
	--namespace "${namespace}" \
	--include-crds >"${helm_manifest}"

(
	cd "${ROOT_DIR}"
	go run ./cmd/packagehelm verify \
		--package-manifest "${package_manifest}" \
		--helm-manifest "${helm_manifest}"
	go run ./cmd/packagehelm security \
		--helm-manifest "${helm_manifest}"
)

(
	cd "${ROOT_DIR}"
	go run ./cmd/packagehelm archive \
		--chart-dir "${chart_dir}" \
		--output "${archive}"
	go run ./cmd/packagehelm checksum --file "${archive}"
)
"${helm_bin}" show chart "${archive}" >/dev/null

echo "Helm chart: ${archive}"
echo "Chart checksum: ${archive}.sha256"
echo "Versioned CRD: ${companion_crd}"
echo "CRD checksum: ${companion_crd}.sha256"
