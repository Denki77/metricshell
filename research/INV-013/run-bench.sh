#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
PROTO="${ROOT_DIR}/prototype"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
RESULTS_DIR="${ROOT_DIR}/results/${RUN_ID}"
RESULTS_REF="research/INV-013/results/${RUN_ID}"
VERSION="0.13.0-research"
GOLANG_IMAGE="golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f"
PHP_IMAGE="php:8.3-cli-alpine@sha256:afdf8b1fee58486ccc0dab5f30f634b86873d56dac985f71ba217945647c05ad"
ALPINE_IMAGE="alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc"
BINFMT_IMAGE="tonistiigi/binfmt:qemu-v9.2.2@sha256:1b804311fe87047a4c96d38b4b3ef6f62fca8cd125265917a9e3dc3c996c39e6"
mkdir -p "${RESULTS_DIR}/artifacts"
ASSERTIONS="${RESULTS_DIR}/assertions.tsv"; OBSERVATIONS="${RESULTS_DIR}/observations.tsv"; SUMMARY="${RESULTS_DIR}/summary.tsv"
printf 'case\tassertion\texpected\tactual\tresult\n' >"${ASSERTIONS}"
printf 'case\tmetric\tvalue\tunit\tnote\n' >"${OBSERVATIONS}"
printf 'case\tresult\tdetails\n' >"${SUMMARY}"
printf '%s\n' "${RESULTS_REF}" >"${ROOT_DIR}/latest-results.txt"
CURRENT_PHASE=initialization
ERROR_DETAIL=

redact_path(){ sed -E "s#${REPO_DIR}#<repo>#g; s#${ROOT_DIR}#<inv013>#g; s#unix:///[^[:space:];,]+#unix://<socket>#g; s#Path:[[:space:]]+/[^[:space:]]+#Path: <redacted>#g; s#(^|[[:space:]])/(Users|home|private|tmp|var)/[^[:space:];,]+#\\1<path>#g"; }
normalize_preflight_error(){
  local label="$1" log="$2" raw
  raw="$(cat "$log")"
  if printf '%s' "$raw"|grep -Eqi 'permission denied|access denied'; then
    ERROR_DETAIL="Docker daemon access denied; docker info must succeed for the current user"
  elif printf '%s' "$raw"|grep -Eqi 'failed to connect|cannot connect|daemon is not running|no such file or directory'; then
    ERROR_DETAIL="Docker daemon is unreachable; start Docker and verify docker info as the current user"
  else
    ERROR_DETAIL="$(printf '%s' "$raw"|tail -n 8|tr '\t\n' '  '|redact_path)"
  fi
  [[ -n "$ERROR_DETAIL" ]]||ERROR_DETAIL="$label failed without diagnostic output"
  printf 'preflight_error\t%s\n' "$ERROR_DETAIL" >"$log"
}
on_error(){
  local code=$? line="$1" command="$2"
  trap - ERR
  set +e
  command="${command//$'\t'/ }"
  command="${command//$'\n'/ }"
  command="$(printf '%s' "$command" | redact_path)"
  ERROR_DETAIL="$(printf '%s' "${ERROR_DETAIL:-not available}"|redact_path)"
  printf 'error\tfail\tphase=%s; exit_code=%s; detail=%s\n' "$CURRENT_PHASE" "$code" "$ERROR_DETAIL" >>"${SUMMARY}"
  printf 'status\terror\nexit_code\t%s\nphase\t%s\nline\t%s\ncommand\t%s\ndetail\t%s\nresults_dir\t%s\n' "$code" "$CURRENT_PHASE" "$line" "$command" "$ERROR_DETAIL" "$RESULTS_REF" >"${RESULTS_DIR}/run-summary.tsv"
  printf 'phase\tline\texit_code\tcommand\tdetail\n%s\t%s\t%s\t%s\t%s\n' "$CURRENT_PHASE" "$line" "$code" "$command" "$ERROR_DETAIL" >"${RESULTS_DIR}/failure.tsv"
  printf 'INV-013 failed during phase %s (exit %s): %s\nEvidence: %s\n' "$CURRENT_PHASE" "$code" "$ERROR_DETAIL" "$RESULTS_REF" >&2
  exit "$code"
}
require_command(){ command -v "$1" >/dev/null 2>&1 || { ERROR_DETAIL="missing prerequisite: $1"; printf 'INV-013 preflight failed: %s\n' "$ERROR_DETAIL" >&2; return 127; }; }
run_preflight(){
  local label="$1" log="$2" code; shift 2
  if "$@" >"$log" 2>&1; then return 0; else code=$?; fi
  normalize_preflight_error "$label" "$log"
  printf 'INV-013 preflight failed (%s): %s\n' "$label" "$ERROR_DETAIL" >&2
  return "$code"
}
run_step(){
  local label="$1" log="$2" code raw; shift 2
  ERROR_DETAIL=
  if "$@" >"$log" 2>&1; then return 0; else code=$?; fi
  raw="$(tail -n 12 "$log")"
  ERROR_DETAIL="$(printf '%s' "$raw"|tr '\t\n' '  '|redact_path)"
  [[ -n "$ERROR_DETAIL" ]]||ERROR_DETAIL="$label failed without diagnostic output"
  printf 'step_error\t%s\n' "$ERROR_DETAIL" >"$log"
  printf 'INV-013 step failed (%s): %s\n' "$label" "$ERROR_DETAIL" >&2
  return "$code"
}
trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR

hash_file(){ if command -v sha256sum >/dev/null; then sha256sum "$1"|awk '{print $1}'; else shasum -a 256 "$1"|awk '{print $1}'; fi; }
hash_stdin(){ if command -v sha256sum >/dev/null; then sha256sum|awk '{print $1}'; else shasum -a 256|awk '{print $1}'; fi; }
fingerprint(){ { find "${PROTO}" -type f|sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh"; }|while IFS= read -r f; do printf '%s  %s\n' "$(hash_file "$f")" "${f#${ROOT_DIR}/}"; done|hash_stdin; }
assert_eq(){ local c="$1" a="$2" e="$3" v="$4" r=fail; [[ "$e" == "$v" ]]&&r=pass; printf '%s\t%s\t%s\t%s\t%s\n' "$c" "$a" "$e" "$v" "$r" >>"${ASSERTIONS}"; }
observe(){ printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "${5:-}" >>"${OBSERVATIONS}"; }
finish_case(){ local c="$1" n; n="$(awk -F '\t' -v c="$c" 'NR>1&&$1==c&&$5!="pass"{n++}END{print n+0}' "${ASSERTIONS}")"; [[ "$n" == 0 ]]&&printf '%s\tpass\tall assertions passed\n' "$c" >>"${SUMMARY}"||printf '%s\tfail\t%s assertion(s) failed\n' "$c" "$n" >>"${SUMMARY}"; }

CURRENT_PHASE=preflight
for required_command in docker git awk sed find sort grep; do require_command "$required_command"; done
run_preflight docker "${RESULTS_DIR}/docker-info.log" docker info
run_preflight docker-version "${RESULTS_DIR}/docker-version.log" docker version
run_preflight docker-buildx "${RESULTS_DIR}/docker-buildx-version.log" docker buildx version

CURRENT_PHASE=base_image_resolution
printf 'role\timmutable_reference\tlocal_image_id\tarchitecture\n' >"${RESULTS_DIR}/base-images.tsv"
for item in "golang|${GOLANG_IMAGE}" "php|${PHP_IMAGE}" "alpine|${ALPINE_IMAGE}" "binfmt|${BINFMT_IMAGE}"; do
  role="${item%%|*}"; reference="${item#*|}"
  docker pull "$reference" >"${RESULTS_DIR}/${role}.pull.log" 2>&1
  image_id="$(docker image inspect "$reference" -f '{{.Id}}')"
  image_arch="$(docker image inspect "$reference" -f '{{.Architecture}}')"
  printf '%s\t%s\t%s\t%s\n' "$role" "$reference" "$image_id" "$image_arch" >>"${RESULTS_DIR}/base-images.tsv"
  assert_eq supply_chain "${role}_image_digest_pinned" true "$([[ "$reference" == *@sha256:* ]]&&echo true||echo false)"
done

# Docker Desktop provides emulation by default; a plain Ubuntu Docker Engine commonly does not.
# Register both benchmark architectures so the same command executes the same cases on both hosts.
CURRENT_PHASE=cross_architecture_emulation
docker run --privileged --rm "$BINFMT_IMAGE" --install amd64,arm64 >"${RESULTS_DIR}/binfmt-install.log" 2>&1

CURRENT_PHASE=native_image_builds
build_image(){ local name="$1" file="$2"; docker build --pull=false --build-arg "VERSION=${VERSION}" -f "${PROTO}/docker/${file}" -t "$name" "${PROTO}" >"${RESULTS_DIR}/${name}.build.log" 2>&1; }
build_image metricshell-inv013-binary Dockerfile.binary
build_image metricshell-inv013-multistage Dockerfile.multistage
build_image metricshell-inv013-base Dockerfile.base

CURRENT_PHASE=native_image_execution
for image in metricshell-inv013-binary metricshell-inv013-multistage metricshell-inv013-base; do
  output="$(docker run --rm "$image")"; printf '%s\n' "$output" >"${RESULTS_DIR}/${image}.run.log"
  assert_eq distribution "$image starts" true "$([[ "$output" == metricshell\ research* ]]&&echo true||echo false)"
  assert_eq version_pin "$image version" "$VERSION" "$(docker run --rm "$image" --version)"
  observe image_size "$image" "$(docker image inspect "$image" -f '{{.Size}}')" bytes
done

CURRENT_PHASE=artifact_extraction
primary_export="${RESULTS_DIR}/artifacts/primary-export"
mkdir -p "$primary_export"
run_step artifact-export "${RESULTS_DIR}/artifact-export.log" docker buildx build --pull=false --build-arg "VERSION=${VERSION}" -f "${PROTO}/docker/Dockerfile.binary" --output "type=local,dest=${primary_export}" "${PROTO}"
if [[ ! -s "${primary_export}/metricshell" ]]; then ERROR_DETAIL="BuildKit export completed without the metricshell artifact"; false; fi
mv "${primary_export}/metricshell" "${RESULTS_DIR}/artifacts/metricshell"
rmdir "$primary_export"
chmod +x "${RESULTS_DIR}/artifacts/metricshell"
artifact_sha="$(hash_file "${RESULTS_DIR}/artifacts/metricshell")"; printf '%s  metricshell\n' "$artifact_sha" >"${RESULTS_DIR}/artifacts/SHA256SUMS"
assert_eq standalone_copy artifact_nonempty true "$([[ -s "${RESULTS_DIR}/artifacts/metricshell" ]]&&echo true||echo false)"
assert_eq supply_chain checksum_verifies "$artifact_sha" "$(hash_file "${RESULTS_DIR}/artifacts/metricshell")"

CURRENT_PHASE=non_root_validation
uid_base="$(docker run --rm metricshell-inv013-base | sed -n 's/.*uid=\([0-9]*\).*/\1/p')"
uid_multi="$(docker run --rm metricshell-inv013-multistage | sed -n 's/.*uid=\([0-9]*\).*/\1/p')"
assert_eq non_root base_uid_nonzero true "$([[ "$uid_base" -gt 0 ]]&&echo true||echo false)"
assert_eq non_root multistage_uid_nonzero true "$([[ "$uid_multi" -gt 0 ]]&&echo true||echo false)"

# The same static artifact executes in scratch, musl Alpine and a PHP Alpine application image.
CURRENT_PHASE=static_binary_validation
assert_eq libc scratch_exec true "$(docker run --rm metricshell-inv013-binary >/dev/null&&echo true||echo false)"
assert_eq libc alpine_exec true "$(docker run --rm metricshell-inv013-base >/dev/null&&echo true||echo false)"
assert_eq libc php_alpine_exec true "$(docker run --rm metricshell-inv013-multistage >/dev/null&&echo true||echo false)"
run_step static-link-check "${RESULTS_DIR}/static-check.log" docker run --rm --entrypoint /bin/sh metricshell-inv013-base -c 'output="$(ldd /usr/local/bin/metricshell 2>&1 || true)"; printf "%s\n" "$output"; printf "%s\n" "$output" | grep -Eiq "not a dynamic executable|statically linked|not a valid dynamic program"'
assert_eq libc cgo_free_static true true

CURRENT_PHASE=cross_architecture_builds
printf 'architecture\timage_architecture\texecuted\n' >"${RESULTS_DIR}/architectures.tsv"
for arch in amd64 arm64; do
  tag="metricshell-inv013-${arch}"
  docker buildx build --platform "linux/${arch}" --pull=false --load --build-arg "VERSION=${VERSION}" -f "${PROTO}/docker/Dockerfile.binary" -t "$tag" "${PROTO}" >"${RESULTS_DIR}/${arch}.build.log" 2>&1
  actual="$(docker image inspect "$tag" -f '{{.Architecture}}')"; executed=false
  if docker run --rm "$tag" --version >"${RESULTS_DIR}/${arch}.run.log" 2>&1; then executed=true; fi
  printf '%s\t%s\t%s\n' "$arch" "$actual" "$executed" >>"${RESULTS_DIR}/architectures.tsv"
  assert_eq architecture_${arch} image_architecture "$arch" "$actual"
  assert_eq architecture_${arch} executable_with_emulation true "$executed"
done

# Rebuild the artifact and require byte reproducibility.
CURRENT_PHASE=reproducibility_rebuild
rebuild_export="${RESULTS_DIR}/artifacts/rebuild-export"
mkdir -p "$rebuild_export"
run_step reproducibility-export "${RESULTS_DIR}/rebuild.log" docker buildx build --no-cache --pull=false --build-arg "VERSION=${VERSION}" -f "${PROTO}/docker/Dockerfile.binary" --output "type=local,dest=${rebuild_export}" "${PROTO}"
if [[ ! -s "${rebuild_export}/metricshell" ]]; then ERROR_DETAIL="BuildKit no-cache export completed without the metricshell artifact"; false; fi
mv "${rebuild_export}/metricshell" "${RESULTS_DIR}/artifacts/metricshell-rebuild"
rmdir "$rebuild_export"
rebuild_sha="$(hash_file "${RESULTS_DIR}/artifacts/metricshell-rebuild")"
assert_eq reproducibility binary_sha256 "$artifact_sha" "$rebuild_sha"

CURRENT_PHASE=evidence_capture
for image in metricshell-inv013-binary metricshell-inv013-multistage metricshell-inv013-base; do
  docker image inspect "$image" >"${RESULTS_DIR}/${image}.inspect.json"
done
cat >"${RESULTS_DIR}/sbom.spdx" <<EOF
SPDXVersion: SPDX-2.3
DataLicense: CC0-1.0
SPDXID: SPDXRef-DOCUMENT
DocumentName: metricshell-inv013-${VERSION}
DocumentNamespace: https://metricshell.invalid/research/INV-013/${artifact_sha}
Creator: Tool: research/INV-013/run-bench.sh
Created: $(date -u +%Y-%m-%dT%H:%M:%SZ)
PackageName: metricshell
SPDXID: SPDXRef-Package-MetricShell
PackageVersion: ${VERSION}
PackageChecksum: SHA256: ${artifact_sha}
PackageDownloadLocation: NOASSERTION
FilesAnalyzed: false
EOF
assert_eq sbom spdx_generated true "$([[ -s "${RESULTS_DIR}/sbom.spdx" ]]&&echo true||echo false)"

for c in distribution standalone_copy version_pin supply_chain non_root libc architecture_amd64 architecture_arm64 reproducibility sbom image_size; do finish_case "$c"; done
{
 printf 'key\tvalue\n'; printf 'investigation\tINV-013\n'; printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; printf 'repository_head_sha\t%s\n' "$(git -C "$REPO_DIR" rev-parse HEAD)"; printf 'benchmark_scope_diff_clean\t%s\n' "$(git -C "$REPO_DIR" diff --quiet -- research/INV-013/prototype research/INV-013/run-bench.sh&&git -C "$REPO_DIR" diff --cached --quiet -- research/INV-013/prototype research/INV-013/run-bench.sh&&echo true||echo false)"; printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"; printf 'host_uname\t%s\n' "$(uname -a|tr '\t\n' ' ')"; printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"; printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"; printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint /metricshell metricshell-inv013-binary --version >/dev/null; docker run --rm "$ALPINE_IMAGE" uname -a)"; printf 'golang_image\t%s\n' "$GOLANG_IMAGE"; printf 'php_image\t%s\n' "$PHP_IMAGE"; printf 'alpine_image\t%s\n' "$ALPINE_IMAGE"; printf 'artifact_sha256\t%s\n' "$artifact_sha";
 printf 'binfmt_image\t%s\n' "$BINFMT_IMAGE";
} >"${RESULTS_DIR}/environment.tsv"
CURRENT_PHASE=final_assertions
failed="$(awk -F '\t' 'NR>1&&$5!="pass"{n++}END{print n+0}' "${ASSERTIONS}")"; printf 'status\tcomplete\nfailed_assertions\t%s\nresults_dir\t%s\n' "$failed" "$RESULTS_REF" >"${RESULTS_DIR}/run-summary.tsv"; [[ "$failed" == 0 ]]
