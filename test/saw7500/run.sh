#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HARNESS_DIR="${ROOT_DIR}/test/saw7500"
TEMPLATES_DIR="${HARNESS_DIR}/templates"

CLUSTER="saw-7500"
NAMESPACE="saw-7500"
RED_IMAGE="public.ecr.aws/s7a5m1b4/sawmills-collector:1.936.0"
GREEN_IMAGE="otelcontribcol-dev:saw-7500"
FAKE_BACKEND_IMAGE="saw7500-fakebackend:latest"
LOADGEN_IMAGE=""
LB_REPLICAS="2"
WORKERS="4"
TARGET_COMPRESSED_BYTES="262144"
MAX_COMPRESSED_BYTES="16777216"
MAX_UNCOMPRESSED_BATCH_BYTES="16777216"
MAX_INFLIGHT_UNCOMPRESSED_BYTES="536870912"
NUM_CONSUMERS="30"
RECEIVER_MAX_RECV_MSG_SIZE_MIB="32"
BACKEND_MAX_RECV_MSG_SIZE_MIB="64"
LOAD_DURATION_SECONDS="720"
WARMUP_SECONDS="120"
SETTLE_SECONDS="180"
SCRAPE_INTERVAL_SECONDS="10"
SCRAPE_CURL_CONNECT_TIMEOUT_SECONDS="2"
SCRAPE_CURL_MAX_TIME_SECONDS="5"
LOAD_RATE="250"
LOAD_WORKERS="8"
BATCH_SIZE="100"
LOAD_SIZE_MB="0"
LOG_BODY="bigid-local-saw-7500-log"
PAYLOAD_PROFILE="repeated"
PAYLOAD_SIZE_BYTES="0"
PAYLOAD_RANDOM_SEED="1"
OTLP_COMPRESSION=""
BACKEND_SLOW_SLEEP="6s"
BACKEND_SLOW_FOR_SECONDS="180"
BACKEND_SLOW_MODULO="4"
BACKEND_SLOW_REMAINDER="3"
BACKEND_UNREADY_FOR_SECONDS="0"
BACKEND_TIMEOUT="5s"
GREEN_MAX_OVER_2S_COUNT="0"
LIVENESS_TIMEOUT_SECONDS="1"
LIVENESS_FAILURE_THRESHOLD="5"
RED_BACKEND_TIMEOUT=""
GREEN_BACKEND_TIMEOUT=""
RED_LIVENESS_TIMEOUT_SECONDS=""
RED_LIVENESS_FAILURE_THRESHOLD=""
GREEN_LIVENESS_TIMEOUT_SECONDS=""
GREEN_LIVENESS_FAILURE_THRESHOLD=""
LB_CPU_REQUEST=""
LB_CPU_LIMIT=""
LB_MEMORY_REQUEST=""
LB_MEMORY_LIMIT=""
LB_GO_MEMLIMIT=""
LB_GOGC=""
REQUIRE_RED_LIVENESS_RESTART="false"
STRICT_RED="false"
ARTIFACT_ROOT="${ROOT_DIR}/artifacts/saw-7500/$(date -u +%Y%m%dT%H%M%SZ)"
PHASE="both"

usage() {
  sed -n '1,80p' "${HARNESS_DIR}/README.md"
}

die() {
  echo "error: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --red-image) RED_IMAGE="$2"; shift 2 ;;
    --green-image) GREEN_IMAGE="$2"; shift 2 ;;
    --fake-backend-image) FAKE_BACKEND_IMAGE="$2"; shift 2 ;;
    --loadgen-image) LOADGEN_IMAGE="$2"; shift 2 ;;
    --telemetrygen-image) LOADGEN_IMAGE="$2"; shift 2 ;;
    --lb-replicas) LB_REPLICAS="$2"; shift 2 ;;
    --workers) WORKERS="$2"; shift 2 ;;
    --target-compressed-bytes) TARGET_COMPRESSED_BYTES="$2"; shift 2 ;;
    --max-compressed-bytes) MAX_COMPRESSED_BYTES="$2"; shift 2 ;;
    --max-uncompressed-batch-bytes) MAX_UNCOMPRESSED_BATCH_BYTES="$2"; shift 2 ;;
    --max-inflight-uncompressed-bytes) MAX_INFLIGHT_UNCOMPRESSED_BYTES="$2"; shift 2 ;;
    --num-consumers) NUM_CONSUMERS="$2"; shift 2 ;;
    --receiver-max-recv-msg-size-mib) RECEIVER_MAX_RECV_MSG_SIZE_MIB="$2"; shift 2 ;;
    --backend-max-recv-msg-size-mib) BACKEND_MAX_RECV_MSG_SIZE_MIB="$2"; shift 2 ;;
    --load-duration-seconds) LOAD_DURATION_SECONDS="$2"; shift 2 ;;
    --warmup-seconds) WARMUP_SECONDS="$2"; shift 2 ;;
    --settle-seconds) SETTLE_SECONDS="$2"; shift 2 ;;
    --scrape-interval-seconds) SCRAPE_INTERVAL_SECONDS="$2"; shift 2 ;;
    --load-rate) LOAD_RATE="$2"; shift 2 ;;
    --load-workers) LOAD_WORKERS="$2"; shift 2 ;;
    --batch-size) BATCH_SIZE="$2"; shift 2 ;;
    --load-size-mb) LOAD_SIZE_MB="$2"; shift 2 ;;
    --payload-profile) PAYLOAD_PROFILE="$2"; shift 2 ;;
    --payload-size-bytes) PAYLOAD_SIZE_BYTES="$2"; shift 2 ;;
    --payload-random-seed) PAYLOAD_RANDOM_SEED="$2"; shift 2 ;;
    --otlp-compression) OTLP_COMPRESSION="$2"; shift 2 ;;
    --backend-slow-sleep) BACKEND_SLOW_SLEEP="$2"; shift 2 ;;
    --backend-slow-for-seconds) BACKEND_SLOW_FOR_SECONDS="$2"; shift 2 ;;
    --backend-slow-modulo) BACKEND_SLOW_MODULO="$2"; shift 2 ;;
    --backend-slow-remainder) BACKEND_SLOW_REMAINDER="$2"; shift 2 ;;
    --backend-unready-for-seconds) BACKEND_UNREADY_FOR_SECONDS="$2"; shift 2 ;;
    --backend-timeout) BACKEND_TIMEOUT="$2"; shift 2 ;;
    --green-max-over-2s-count) GREEN_MAX_OVER_2S_COUNT="$2"; shift 2 ;;
    --red-backend-timeout) RED_BACKEND_TIMEOUT="$2"; shift 2 ;;
    --green-backend-timeout) GREEN_BACKEND_TIMEOUT="$2"; shift 2 ;;
    --liveness-timeout-seconds) LIVENESS_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --liveness-failure-threshold) LIVENESS_FAILURE_THRESHOLD="$2"; shift 2 ;;
    --red-liveness-timeout-seconds) RED_LIVENESS_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --red-liveness-failure-threshold) RED_LIVENESS_FAILURE_THRESHOLD="$2"; shift 2 ;;
    --green-liveness-timeout-seconds) GREEN_LIVENESS_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --green-liveness-failure-threshold) GREEN_LIVENESS_FAILURE_THRESHOLD="$2"; shift 2 ;;
    --lb-cpu-request) LB_CPU_REQUEST="$2"; shift 2 ;;
    --lb-cpu-limit) LB_CPU_LIMIT="$2"; shift 2 ;;
    --lb-memory-request) LB_MEMORY_REQUEST="$2"; shift 2 ;;
    --lb-memory-limit) LB_MEMORY_LIMIT="$2"; shift 2 ;;
    --lb-go-memlimit) LB_GO_MEMLIMIT="$2"; shift 2 ;;
    --lb-gogc) LB_GOGC="$2"; shift 2 ;;
    --artifacts) ARTIFACT_ROOT="$2"; shift 2 ;;
    --phase) PHASE="$2"; shift 2 ;;
    --require-red-liveness-restart) REQUIRE_RED_LIVENESS_RESTART="true"; shift ;;
    --no-red-liveness-restart-required) REQUIRE_RED_LIVENESS_RESTART="false"; shift ;;
    --strict-red) STRICT_RED="true"; shift ;;
    --red-any-signature) STRICT_RED="false"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument $1" ;;
  esac
done

command -v kubectl >/dev/null || die "kubectl is required"
command -v docker >/dev/null || die "docker is required"
command -v curl >/dev/null || die "curl is required"
command -v python3 >/dev/null || die "python3 is required"
command -v kind >/dev/null || die "kind is required; install it, then rerun"

duration() {
  printf "%ss" "$1"
}

image_is_local_name() {
  [[ "$1" != *"/"* ]]
}

load_image_if_local() {
  local image="$1"
  if docker image inspect "${image}" >/dev/null 2>&1; then
    echo "loading local image into kind: ${image}"
    kind load docker-image "${image}" --name "${CLUSTER}"
    return
  fi
  if image_is_local_name "${image}"; then
    die "local image ${image} was not found; build or retag it first"
  fi
  echo "image ${image} not local; kind will pull it from registry"
}

ensure_cluster() {
  if kind get clusters -q | grep -Fxq "${CLUSTER}"; then
    echo "kind cluster ${CLUSTER} exists"
    return
  fi
  kind create cluster --name "${CLUSTER}" --config "${HARNESS_DIR}/kind.yaml"
}

render_templates() {
  local phase="$1"
  local collector_image="$2"
  local endpoint_health_enabled="$3"
  local active_probe_enabled="$4"
  local backend_timeout="$5"
  local liveness_timeout_seconds="$6"
  local liveness_failure_threshold="$7"
  local out_dir="$8"

  mkdir -p "${out_dir}"

  export WORKERS
  export LB_REPLICAS
  export FAKE_BACKEND_IMAGE
  export LOADGEN_IMAGE
  export COLLECTOR_IMAGE="${collector_image}"
  export TARGET_COMPRESSED_BYTES
  export MAX_COMPRESSED_BYTES
  export MAX_UNCOMPRESSED_BATCH_BYTES
  export MAX_INFLIGHT_UNCOMPRESSED_BYTES
  export NUM_CONSUMERS
  export NUM_CONSUMERS_CONFIG=""
  if [[ "${phase}" == "green" ]]; then
    NUM_CONSUMERS_CONFIG="          num_consumers: ${NUM_CONSUMERS}"
  fi
  export RECEIVER_MAX_RECV_MSG_SIZE_MIB
  export BACKEND_MAX_RECV_MSG_SIZE_MIB
  export BACKEND_SLOW_SLEEP
  export BACKEND_SLOW_MODULO
  export BACKEND_SLOW_REMAINDER
  export BACKEND_TIMEOUT="${backend_timeout}"
  export LOAD_DURATION="$(duration "${LOAD_DURATION_SECONDS}")"
  export LOAD_RATE
  export LOAD_WORKERS
  export BATCH_SIZE
  export LOAD_SIZE_MB
  export LOG_BODY
  export PAYLOAD_PROFILE
  export PAYLOAD_SIZE_BYTES
  export PAYLOAD_RANDOM_SEED
  export OTLP_COMPRESSION
  export ENDPOINT_HEALTH_ENABLED="${endpoint_health_enabled}"
  export ACTIVE_PROBE_ENABLED="${active_probe_enabled}"
  export LIVENESS_TIMEOUT_SECONDS="${liveness_timeout_seconds}"
  export LIVENESS_FAILURE_THRESHOLD="${liveness_failure_threshold}"
  export LB_ENV=""
  if [[ -n "${LB_GO_MEMLIMIT}" || -n "${LB_GOGC}" ]]; then
    LB_ENV="          env:"
    if [[ -n "${LB_GO_MEMLIMIT}" ]]; then
      LB_ENV+=$'\n            - name: GOMEMLIMIT'
      LB_ENV+=$'\n              value: "'"${LB_GO_MEMLIMIT}"'"'
    fi
    if [[ -n "${LB_GOGC}" ]]; then
      LB_ENV+=$'\n            - name: GOGC'
      LB_ENV+=$'\n              value: "'"${LB_GOGC}"'"'
    fi
  fi
  export LB_RESOURCES=""
  if [[ -n "${LB_CPU_REQUEST}" || -n "${LB_CPU_LIMIT}" || -n "${LB_MEMORY_REQUEST}" || -n "${LB_MEMORY_LIMIT}" ]]; then
    LB_RESOURCES="          resources:"
    if [[ -n "${LB_CPU_REQUEST}" || -n "${LB_MEMORY_REQUEST}" ]]; then
      LB_RESOURCES+=$'\n            requests:'
      if [[ -n "${LB_CPU_REQUEST}" ]]; then
        LB_RESOURCES+=$'\n              cpu: '"${LB_CPU_REQUEST}"
      fi
      if [[ -n "${LB_MEMORY_REQUEST}" ]]; then
        LB_RESOURCES+=$'\n              memory: '"${LB_MEMORY_REQUEST}"
      fi
    fi
    if [[ -n "${LB_CPU_LIMIT}" || -n "${LB_MEMORY_LIMIT}" ]]; then
      LB_RESOURCES+=$'\n            limits:'
      if [[ -n "${LB_CPU_LIMIT}" ]]; then
        LB_RESOURCES+=$'\n              cpu: '"${LB_CPU_LIMIT}"
      fi
      if [[ -n "${LB_MEMORY_LIMIT}" ]]; then
        LB_RESOURCES+=$'\n              memory: '"${LB_MEMORY_LIMIT}"
      fi
    fi
  fi

  cp "${TEMPLATES_DIR}/namespace.yaml" "${out_dir}/00-namespace.yaml"
  cp "${TEMPLATES_DIR}/lb-rbac.yaml" "${out_dir}/01-lb-rbac.yaml"

  for template in backend lb-config lb loadgen-job; do
    perl -pe '
      s/__WORKERS__/$ENV{WORKERS}/g;
      s/__LB_REPLICAS__/$ENV{LB_REPLICAS}/g;
      s#__FAKE_BACKEND_IMAGE__#$ENV{FAKE_BACKEND_IMAGE}#g;
      s#__LOADGEN_IMAGE__#$ENV{LOADGEN_IMAGE}#g;
      s#__COLLECTOR_IMAGE__#$ENV{COLLECTOR_IMAGE}#g;
      s/__TARGET_COMPRESSED_BYTES__/$ENV{TARGET_COMPRESSED_BYTES}/g;
      s/__MAX_COMPRESSED_BYTES__/$ENV{MAX_COMPRESSED_BYTES}/g;
      s/__MAX_UNCOMPRESSED_BATCH_BYTES__/$ENV{MAX_UNCOMPRESSED_BATCH_BYTES}/g;
      s/__MAX_INFLIGHT_UNCOMPRESSED_BYTES__/$ENV{MAX_INFLIGHT_UNCOMPRESSED_BYTES}/g;
      s#__NUM_CONSUMERS_CONFIG__#$ENV{NUM_CONSUMERS_CONFIG}#g;
      s/__RECEIVER_MAX_RECV_MSG_SIZE_MIB__/$ENV{RECEIVER_MAX_RECV_MSG_SIZE_MIB}/g;
      s/__BACKEND_MAX_RECV_MSG_SIZE_MIB__/$ENV{BACKEND_MAX_RECV_MSG_SIZE_MIB}/g;
      s/__BACKEND_SLOW_SLEEP__/$ENV{BACKEND_SLOW_SLEEP}/g;
      s/__BACKEND_SLOW_MODULO__/$ENV{BACKEND_SLOW_MODULO}/g;
      s/__BACKEND_SLOW_REMAINDER__/$ENV{BACKEND_SLOW_REMAINDER}/g;
      s/__BACKEND_TIMEOUT__/$ENV{BACKEND_TIMEOUT}/g;
      s/__LOAD_DURATION__/$ENV{LOAD_DURATION}/g;
      s/__LOAD_RATE__/$ENV{LOAD_RATE}/g;
      s/__LOAD_WORKERS__/$ENV{LOAD_WORKERS}/g;
      s/__BATCH_SIZE__/$ENV{BATCH_SIZE}/g;
      s/__LOAD_SIZE_MB__/$ENV{LOAD_SIZE_MB}/g;
      s/__LOG_BODY__/$ENV{LOG_BODY}/g;
      s/__PAYLOAD_PROFILE__/$ENV{PAYLOAD_PROFILE}/g;
      s/__PAYLOAD_SIZE_BYTES__/$ENV{PAYLOAD_SIZE_BYTES}/g;
      s/__PAYLOAD_RANDOM_SEED__/$ENV{PAYLOAD_RANDOM_SEED}/g;
      s/__OTLP_COMPRESSION__/$ENV{OTLP_COMPRESSION}/g;
      s/__ENDPOINT_HEALTH_ENABLED__/$ENV{ENDPOINT_HEALTH_ENABLED}/g;
      s/__ACTIVE_PROBE_ENABLED__/$ENV{ACTIVE_PROBE_ENABLED}/g;
      s/__LIVENESS_TIMEOUT_SECONDS__/$ENV{LIVENESS_TIMEOUT_SECONDS}/g;
      s/__LIVENESS_FAILURE_THRESHOLD__/$ENV{LIVENESS_FAILURE_THRESHOLD}/g;
      s#__LB_ENV__#$ENV{LB_ENV}#g;
      s#__LB_RESOURCES__#$ENV{LB_RESOURCES}#g;
    ' "${TEMPLATES_DIR}/${template}.yaml.tmpl" > "${out_dir}/${template}.yaml"
  done

  printf "phase=%s\ncollector_image=%s\nendpoint_health=%s\nactive_probe=%s\nworkers=%s\nlb_replicas=%s\ntarget_compressed_bytes=%s\nmax_compressed_bytes=%s\nmax_uncompressed_batch_bytes=%s\nmax_inflight_uncompressed_bytes=%s\nnum_consumers=%s\nreceiver_max_recv_msg_size_mib=%s\nbackend_max_recv_msg_size_mib=%s\nload_duration_seconds=%s\nwarmup_seconds=%s\nsettle_seconds=%s\nscrape_interval_seconds=%s\nload_rate=%s\nload_workers=%s\nbatch_size=%s\nload_size_mb=%s\npayload_profile=%s\npayload_size_bytes=%s\npayload_random_seed=%s\notlp_compression=%s\nbackend_timeout=%s\nbackend_slow_sleep=%s\nbackend_slow_for_seconds=%s\nbackend_slow_modulo=%s\nbackend_slow_remainder=%s\nbackend_unready_for_seconds=%s\nliveness_timeout_seconds=%s\nliveness_failure_threshold=%s\nlb_cpu_request=%s\nlb_cpu_limit=%s\nlb_memory_request=%s\nlb_memory_limit=%s\nlb_go_memlimit=%s\nlb_gogc=%s\nstrict_red=%s\nrequire_red_liveness_restart=%s\n" \
    "${phase}" "${collector_image}" "${endpoint_health_enabled}" "${active_probe_enabled}" \
    "${WORKERS}" "${LB_REPLICAS}" "${TARGET_COMPRESSED_BYTES}" "${MAX_COMPRESSED_BYTES}" \
    "${MAX_UNCOMPRESSED_BATCH_BYTES}" "${MAX_INFLIGHT_UNCOMPRESSED_BYTES}" \
    "${NUM_CONSUMERS}" \
    "${RECEIVER_MAX_RECV_MSG_SIZE_MIB}" "${BACKEND_MAX_RECV_MSG_SIZE_MIB}" \
    "${LOAD_DURATION_SECONDS}" "${WARMUP_SECONDS}" "${SETTLE_SECONDS}" "${SCRAPE_INTERVAL_SECONDS}" \
    "${LOAD_RATE}" "${LOAD_WORKERS}" "${BATCH_SIZE}" "${LOAD_SIZE_MB}" \
    "${PAYLOAD_PROFILE}" "${PAYLOAD_SIZE_BYTES}" "${PAYLOAD_RANDOM_SEED}" "${OTLP_COMPRESSION}" \
    "${backend_timeout}" \
    "${BACKEND_SLOW_SLEEP}" "${BACKEND_SLOW_FOR_SECONDS}" \
    "${BACKEND_SLOW_MODULO}" "${BACKEND_SLOW_REMAINDER}" "${BACKEND_UNREADY_FOR_SECONDS}" \
    "${liveness_timeout_seconds}" "${liveness_failure_threshold}" \
    "${LB_CPU_REQUEST}" "${LB_CPU_LIMIT}" "${LB_MEMORY_REQUEST}" "${LB_MEMORY_LIMIT}" \
    "${LB_GO_MEMLIMIT}" "${LB_GOGC}" "${STRICT_RED}" "${REQUIRE_RED_LIVENESS_RESTART}" > "${out_dir}/render-vars.txt"
}

kubectl_apply_dir() {
  local dir="$1"
  for manifest in "${dir}"/*.yaml; do
    kubectl apply -f "${manifest}" || return 1
  done
}

kubectl_apply_infra_dir() {
  local dir="$1"
  for manifest in "${dir}"/*.yaml; do
    [[ "$(basename "${manifest}")" == "loadgen-job.yaml" ]] && continue
    kubectl apply -f "${manifest}" || return 1
  done
}

delete_namespace() {
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false || return 1
  local deadline=$((SECONDS + 180))
  while kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      echo "timed out waiting for namespace ${NAMESPACE} deletion" >&2
      return 1
    fi
    sleep 1
  done
}

scrape_pod_metrics() {
  local namespace="$1"
  local selector="$2"
  local remote_port="$3"
  local prefix="$4"
  local tick="$5"
  local out_dir="$6"
  local pods
  pods="$(kubectl get pods -n "${namespace}" -l "${selector}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  [[ -n "${pods}" ]] || return 0

  local index=0
  while IFS= read -r pod; do
    [[ -n "${pod}" ]] || continue
    local local_port=$((19000 + (RANDOM % 20000)))
    kubectl port-forward -n "${namespace}" "pod/${pod}" "${local_port}:${remote_port}" --address 127.0.0.1 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 0.4
    curl -fsS \
      --connect-timeout "${SCRAPE_CURL_CONNECT_TIMEOUT_SECONDS}" \
      --max-time "${SCRAPE_CURL_MAX_TIME_SECONDS}" \
      "http://127.0.0.1:${local_port}/metrics" > "${out_dir}/${prefix}-${tick}-${index}-${pod}.prom" || true
    kill "${pf_pid}" >/dev/null 2>&1 || true
    wait "${pf_pid}" >/dev/null 2>&1 || true
    index=$((index + 1))
  done <<< "${pods}"
}

scrape_service_metrics() {
  local namespace="$1"
  local service="$2"
  local remote_port="$3"
  local prefix="$4"
  local tick="$5"
  local out_dir="$6"

  local local_port=$((19000 + (RANDOM % 20000)))
  kubectl port-forward -n "${namespace}" "svc/${service}" "${local_port}:${remote_port}" --address 127.0.0.1 >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 0.4
  curl -fsS \
    --connect-timeout "${SCRAPE_CURL_CONNECT_TIMEOUT_SECONDS}" \
    --max-time "${SCRAPE_CURL_MAX_TIME_SECONDS}" \
    "http://127.0.0.1:${local_port}/metrics" > "${out_dir}/${prefix}-${tick}-0-${service}.prom" || true
  kill "${pf_pid}" >/dev/null 2>&1 || true
  wait "${pf_pid}" >/dev/null 2>&1 || true
}

scrape_loop() {
  local out_dir="$1"
  local seconds="$2"
  local interval="${SCRAPE_INTERVAL_SECONDS}"
  local end=$((SECONDS + seconds))
  local tick=0
  while (( SECONDS < end )); do
    local tick_id
    tick_id="$(printf "%06d" "${tick}")"
    scrape_pod_metrics "${NAMESPACE}" "app=saw-7500-lb" 8888 "lb-metrics" "${tick_id}" "${out_dir}"
    scrape_pod_metrics "${NAMESPACE}" "app=saw-7500-backend" 13133 "backend-metrics" "${tick_id}" "${out_dir}"
    scrape_service_metrics "${NAMESPACE}" "saw-7500-tally" 13134 "tally-metrics" "${tick_id}" "${out_dir}"
    tick=$((tick + 1))
    sleep "${interval}"
  done
}

capture_k8s() {
  local out_dir="$1"
  local suffix="$2"
  kubectl get pods -n "${NAMESPACE}" -o wide > "${out_dir}/pods-${suffix}.txt" || true
  kubectl get pods -n "${NAMESPACE}" -l app=saw-7500-lb -o json > "${out_dir}/pods-${suffix}.json" || true
  kubectl get events -n "${NAMESPACE}" --sort-by=.lastTimestamp > "${out_dir}/events-${suffix}.txt" || true
  kubectl get endpointslices -n "${NAMESPACE}" -o yaml > "${out_dir}/endpointslices-${suffix}.yaml" || true
}

run_phase() {
  local phase="$1"
  local collector_image="$2"
  local endpoint_health_enabled="$3"
  local active_probe_enabled="$4"
  local backend_timeout="$5"
  local liveness_timeout_seconds="$6"
  local liveness_failure_threshold="$7"
  local out_dir="${ARTIFACT_ROOT}/${phase}"
  local render_dir="${out_dir}/rendered"
  mkdir -p "${out_dir}"

  echo "=== ${phase}: apply manifests ==="
  delete_namespace || return 1
  render_templates "${phase}" "${collector_image}" "${endpoint_health_enabled}" "${active_probe_enabled}" \
    "${backend_timeout}" "${liveness_timeout_seconds}" "${liveness_failure_threshold}" "${render_dir}" || return 1
  kubectl_apply_infra_dir "${render_dir}" || return 1

  kubectl rollout status -n "${NAMESPACE}" statefulset/saw-7500-backend --timeout=180s || return 1
  kubectl rollout status -n "${NAMESPACE}" deployment/saw-7500-tally --timeout=180s || return 1
  kubectl rollout status -n "${NAMESPACE}" deployment/saw-7500-lb --timeout=180s || return 1
  capture_k8s "${out_dir}" "before"

  local scrape_seconds=$((LOAD_DURATION_SECONDS + SETTLE_SECONDS + 60))
  scrape_loop "${out_dir}" "${scrape_seconds}" &
  local scrape_pid=$!

  echo "=== ${phase}: start loadgen ==="
  kubectl apply -f "${render_dir}/loadgen-job.yaml" || return 1

  echo "=== ${phase}: warmup ${WARMUP_SECONDS}s ==="
  sleep "${WARMUP_SECONDS}"

  echo "=== ${phase}: inject backend rollout churn ==="
  local unready_after="0s"
  if (( BACKEND_UNREADY_FOR_SECONDS > 0 )); then
    unready_after="1ns"
  fi
  kubectl set env -n "${NAMESPACE}" statefulset/saw-7500-backend \
    SLOW_AFTER=1ns \
    SLOW_FOR="$(duration "${BACKEND_SLOW_FOR_SECONDS}")" \
    UNREADY_AFTER="${unready_after}" \
    UNREADY_FOR="$(duration "${BACKEND_UNREADY_FOR_SECONDS}")" \
    ROLLOUT_ID="$(date -u +%s)" \
    > "${out_dir}/rollout-patch.txt" || return 1

  local rollout_timeout=$((LOAD_DURATION_SECONDS + SETTLE_SECONDS))
  if kubectl rollout status -n "${NAMESPACE}" statefulset/saw-7500-backend --timeout="${rollout_timeout}s" > "${out_dir}/rollout-status.txt" 2>&1; then
    echo "backend rollout completed"
  else
    echo "backend rollout did not complete; see ${out_dir}/rollout-status.txt"
  fi

  kubectl wait -n "${NAMESPACE}" --for=condition=complete job/saw-7500-loadgen --timeout="$((LOAD_DURATION_SECONDS + SETTLE_SECONDS))s" \
    > "${out_dir}/loadgen-wait.txt" 2>&1 || true
  kubectl logs -n "${NAMESPACE}" job/saw-7500-loadgen > "${out_dir}/loadgen.log" 2>&1 || true

  echo "=== ${phase}: settle ${SETTLE_SECONDS}s ==="
  sleep "${SETTLE_SECONDS}"

  kill "${scrape_pid}" >/dev/null 2>&1 || true
  wait "${scrape_pid}" >/dev/null 2>&1 || true
  scrape_pod_metrics "${NAMESPACE}" "app=saw-7500-lb" 8888 "lb-metrics" "999999" "${out_dir}"
  scrape_pod_metrics "${NAMESPACE}" "app=saw-7500-backend" 13133 "backend-metrics" "999999" "${out_dir}"
  scrape_service_metrics "${NAMESPACE}" "saw-7500-tally" 13134 "tally-metrics" "999999" "${out_dir}"
  capture_k8s "${out_dir}" "after"

  local analyzer_args=(
    "${HARNESS_DIR}/analyze.py"
    --artifacts "${out_dir}"
    --expect "${phase}"
    --queue-capacity-bytes "${MAX_COMPRESSED_BYTES}"
    --queue-budget-bytes "${MAX_COMPRESSED_BYTES}"
    --latency-timeout-ms 5000
    --green-max-over-2s-count "${GREEN_MAX_OVER_2S_COUNT}"
  )
  if [[ "${phase}" == "red" && "${REQUIRE_RED_LIVENESS_RESTART}" == "true" ]]; then
    analyzer_args+=(--require-red-liveness-restart)
  fi
  if [[ "${phase}" == "red" && "${STRICT_RED}" == "true" ]]; then
    analyzer_args+=(--strict-red)
  fi
  python3 "${analyzer_args[@]}" > "${out_dir}/analyze.log"
}

mkdir -p "${ARTIFACT_ROOT}"
LOADGEN_IMAGE="${LOADGEN_IMAGE:-${FAKE_BACKEND_IMAGE}}"
if [[ "${LOAD_SIZE_MB}" != "0" && "${PAYLOAD_SIZE_BYTES}" == "0" ]]; then
  PAYLOAD_SIZE_BYTES=$((LOAD_SIZE_MB * 1024 * 1024))
fi
ensure_cluster
docker build -t "${FAKE_BACKEND_IMAGE}" "${HARNESS_DIR}/fakebackend"
load_image_if_local "${FAKE_BACKEND_IMAGE}"
load_image_if_local "${LOADGEN_IMAGE}"
load_image_if_local "${RED_IMAGE}"
load_image_if_local "${GREEN_IMAGE}"

set +e
red_backend_timeout="${RED_BACKEND_TIMEOUT:-${BACKEND_TIMEOUT}}"
green_backend_timeout="${GREEN_BACKEND_TIMEOUT:-${BACKEND_TIMEOUT}}"
red_liveness_timeout="${RED_LIVENESS_TIMEOUT_SECONDS:-${LIVENESS_TIMEOUT_SECONDS}}"
red_liveness_failure="${RED_LIVENESS_FAILURE_THRESHOLD:-${LIVENESS_FAILURE_THRESHOLD}}"
green_liveness_timeout="${GREEN_LIVENESS_TIMEOUT_SECONDS:-${LIVENESS_TIMEOUT_SECONDS}}"
green_liveness_failure="${GREEN_LIVENESS_FAILURE_THRESHOLD:-${LIVENESS_FAILURE_THRESHOLD}}"
case "${PHASE}" in
  red)
    run_phase red "${RED_IMAGE}" "false" "false" "${red_backend_timeout}" "${red_liveness_timeout}" "${red_liveness_failure}"
    red_status=$?
    green_status=0
    ;;
  green)
    red_status=0
    run_phase green "${GREEN_IMAGE}" "true" "true" "${green_backend_timeout}" "${green_liveness_timeout}" "${green_liveness_failure}"
    green_status=$?
    ;;
  both)
    run_phase red "${RED_IMAGE}" "false" "false" "${red_backend_timeout}" "${red_liveness_timeout}" "${red_liveness_failure}"
    red_status=$?
    run_phase green "${GREEN_IMAGE}" "true" "true" "${green_backend_timeout}" "${green_liveness_timeout}" "${green_liveness_failure}"
    green_status=$?
    ;;
  *)
    die "unknown phase ${PHASE}; expected red, green, or both"
    ;;
esac
set -e

echo "artifacts: ${ARTIFACT_ROOT}"
echo "red analyzer status: ${red_status}"
echo "green analyzer status: ${green_status}"

if [[ "${red_status}" -ne 0 || "${green_status}" -ne 0 ]]; then
  exit 1
fi
