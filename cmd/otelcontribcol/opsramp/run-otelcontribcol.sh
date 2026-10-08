#!/bin/sh
# Dynamic configuration is copied to a private snapshot before validation. The
# collector always starts from those exact bytes, never the changing shared file.
# Extra arguments bypass the supervisor and are passed directly to the collector.
set -eu

BIN="${OTELCOL_BINARY:-/otelcontribcol}"
DYNAMIC="${OTEL_LOGS_CONFIG_FILE:-/etc/otel-logs-config/collector.json}"
OUTPUT_DIR="${OTELCOL_OUTPUT_DIR:-/var/log/otelcol}"
INTERVAL="${OTELCOL_WATCH_INTERVAL:-5}"
STOP_TIMEOUT="${OTELCOL_STOP_TIMEOUT:-8}"
STARTUP_TIMEOUT="${OTELCOL_STARTUP_TIMEOUT:-15}"

if [ "$#" -gt 0 ]; then
  exec "${BIN}" "$@"
fi

mkdir -p "${OUTPUT_DIR}"
for seconds in "${INTERVAL}" "${STOP_TIMEOUT}" "${STARTUP_TIMEOUT}"; do
  case "${seconds}" in
    ''|*[!0-9]*|0) echo "watch, startup and stop timeouts must be positive integer seconds" >&2; exit 1 ;;
  esac
  if [ "${seconds}" -le 0 ]; then
    echo "watch, startup and stop timeouts must be greater than zero" >&2
    exit 1
  fi
done

umask 077
RUNTIME="$(mktemp -d "${OUTPUT_DIR}/.otel-config.XXXXXX")"
ACTIVE="${RUNTIME}/active.json"
CANDIDATE="${RUNTIME}/candidate.json"
REJECTED="${RUNTIME}/rejected.json"
PREVIOUS="${RUNTIME}/previous.json"
PID=""
VALIDATOR_PID=""
SLEEP_PID=""
WAITING=false

reap_child() {
  # A process terminated by a signal is expected to return a non-zero status.
  if wait "$1" 2>/dev/null; then :; fi
}

stop_child() {
  child="$1"
  if [ -n "${child}" ]; then
    if kill -0 "${child}" 2>/dev/null; then
      if ! kill -TERM "${child}" 2>/dev/null; then
        echo "process ${child} exited before SIGTERM could be delivered" >&2
      fi
      elapsed=0
      while kill -0 "${child}" 2>/dev/null; do
        if [ "${elapsed}" -ge "${STOP_TIMEOUT}" ]; then
          echo "process ${child} exceeded shutdown timeout; sending SIGKILL" >&2
          if ! kill -KILL "${child}" 2>/dev/null; then
            echo "process ${child} exited before SIGKILL could be delivered" >&2
          fi
          break
        fi
        sleep 1
        elapsed=$((elapsed + 1))
      done
    fi
    reap_child "${child}"
  fi
}

cleanup() {
  trap '' TERM INT
  stop_child "${SLEEP_PID}"
  stop_child "${VALIDATOR_PID}"
  stop_child "${PID}"
  rm -f "${ACTIVE}" "${CANDIDATE}" "${REJECTED}" "${PREVIOUS}"
  rmdir "${RUNTIME}"
}
trap cleanup EXIT
trap 'exit 0' TERM INT

validate_candidate() {
  if [ ! -s "${CANDIDATE}" ]; then
    echo "empty collector configuration is invalid" >&2
    return 1
  fi
  "${BIN}" validate --config "${CANDIDATE}" &
  VALIDATOR_PID=$!
  if wait "${VALIDATOR_PID}"; then
    VALIDATOR_PID=""
    return 0
  fi
  VALIDATOR_PID=""
  return 1
}

start_collector() {
  echo "starting otelcontribcol from validated snapshot (${SOURCE})"
  "${BIN}" --config "${ACTIVE}" &
  PID=$!
}

wait_for_startup() {
  elapsed=0
  while [ "${elapsed}" -lt "${STARTUP_TIMEOUT}" ]; do
    sleep 1 &
    SLEEP_PID=$!
    reap_child "${SLEEP_PID}"
    SLEEP_PID=""
    if ! kill -0 "${PID}" 2>/dev/null; then
      return 1
    fi
    if [ -n "${OTEL_LOGS_HEALTH_PORT:-}" ] &&
       wget -q -T 1 -O /dev/null "http://127.0.0.1:${OTEL_LOGS_HEALTH_PORT}/"; then
      return 0
    fi
    elapsed=$((elapsed + 1))
  done
  # Outside the sidecar, no health port means a bounded startup-survival check.
  [ -z "${OTEL_LOGS_HEALTH_PORT:-}" ]
}

# Returns failure without stopping the collector when a candidate cannot be used.
apply_source() {
  SOURCE="$1"
  if ! cp "${SOURCE}" "${CANDIDATE}"; then
    echo "cannot snapshot ${SOURCE}; retaining current configuration" >&2
    return 1
  fi
  if [ -f "${ACTIVE}" ] && cmp -s "${CANDIDATE}" "${ACTIVE}"; then
    return 0
  fi
  if [ -f "${REJECTED}" ] && cmp -s "${CANDIDATE}" "${REJECTED}"; then
    return 1
  fi
  if ! validate_candidate; then
    echo "rejecting invalid config ${SOURCE}; retaining current configuration" >&2
    if ! mv "${CANDIDATE}" "${REJECTED}"; then
      echo "cannot retain rejected snapshot" >&2
    fi
    return 1
  fi

  if [ -f "${ACTIVE}" ]; then
    if ! cp "${ACTIVE}" "${PREVIOUS}"; then
      echo "cannot preserve rollback snapshot; retaining running collector" >&2
      return 1
    fi
  fi
  stop_child "${PID}"
  PID=""
  if ! mv "${CANDIDATE}" "${ACTIVE}"; then
    echo "cannot install validated snapshot" >&2
    exit 1
  fi
  start_collector
  if wait_for_startup; then
    if ! rm -f "${REJECTED}" "${PREVIOUS}"; then
      echo "cannot clean previous configuration snapshots" >&2
      exit 1
    fi
    return 0
  fi
  echo "collector failed startup for ${SOURCE}; rolling back" >&2
  stop_child "${PID}"
  PID=""
  if ! mv "${ACTIVE}" "${REJECTED}"; then
    echo "cannot retain failed startup snapshot" >&2
    exit 1
  fi
  if [ -f "${PREVIOUS}" ]; then
    if ! mv "${PREVIOUS}" "${ACTIVE}"; then
      echo "cannot restore rollback snapshot" >&2
      exit 1
    fi
    SOURCE="last working configuration"
    start_collector
    if ! wait_for_startup; then
      echo "collector rollback failed" >&2
      exit 1
    fi
  fi
  return 1
}

while true; do
  if [ -n "${PID}" ] && ! kill -0 "${PID}" 2>/dev/null; then
    rc=0
    wait "${PID}" || rc=$?
    PID=""
    echo "otelcontribcol exited rc=${rc}" >&2
    exit "${rc}"
  fi

  if [ ! -e "${DYNAMIC}" ] || { [ -f "${DYNAMIC}" ] && [ ! -s "${DYNAMIC}" ]; }; then
    if [ "${WAITING}" = false ]; then
      echo "waiting for non-empty CR-generated configuration at ${DYNAMIC}; collector stopped"
      WAITING=true
    fi
    stop_child "${PID}"
    PID=""
    rm -f "${ACTIVE}" "${CANDIDATE}" "${REJECTED}" "${PREVIOUS}"
  else
    WAITING=false
    if ! apply_source "${DYNAMIC}"; then
      # Invalid replacements keep a running collector; invalid initial configs wait for correction.
      :
    fi
  fi

  sleep "${INTERVAL}" &
  SLEEP_PID=$!
  reap_child "${SLEEP_PID}"
  SLEEP_PID=""
done
