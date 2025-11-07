#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ENV_FILE=${ENV_FILE:-"${SCRIPT_DIR}/vpc_flow_fixtures.env"}
if [[ -f "${ENV_FILE}" ]]; then
  echo "Loading environment from ${ENV_FILE}"
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

# Removes the GCP resources created by generate_vpc_flow_fixtures_phase1_2.sh.
#
# By default the script assumes all artifacts share the RESOURCE_PREFIX used by
# the generator. You may override individual resource names, but they must keep
# the prefix to avoid accidental deletion of unrelated infrastructure.
#
# Usage:
#   PROJECT_ID=my-project REGION=us-central1 \
#   scripts/teardown_vpc_flow_fixtures_phase1_2.sh            # dry-run (default)
#   scripts/teardown_vpc_flow_fixtures_phase1_2.sh --dry-run=false  # actually delete
#
# Optional knobs:
#   RESOURCE_PREFIX (default: gcp-fixture)

DRY_RUN="true"
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --dry-run=false)
      DRY_RUN="false"
      ;;
    --dry-run=true)
      DRY_RUN="true"
      ;;
    --dry-run=*)
      echo "Invalid value for --dry-run. Use true or false." >&2
      exit 1
      ;;
    *)
      ARGS+=("$arg")
      ;;
  esac
done
if (( ${#ARGS[@]} )); then
  set -- "${ARGS[@]}"
else
  set --
fi

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "Teardown running in dry-run mode. No resources will be modified."
else
  echo "Teardown running with modifications enabled."
fi

required_env=(PROJECT_ID REGION)
missing=()
for var in "${required_env[@]}"; do
  if [[ -z "${!var-}" ]]; then
    missing+=("${var}")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'Missing required env vars: %s\n' "${missing[*]}" >&2
  exit 1
fi

command -v gcloud >/dev/null || {
  echo "gcloud CLI is required" >&2
  exit 1
}

RESOURCE_PREFIX=${RESOURCE_PREFIX:-gcp-fixture}
if [[ -z "${RESOURCE_PREFIX}" ]]; then
  echo "RESOURCE_PREFIX cannot be empty" >&2
  exit 1
fi

NETWORK_NAME="${RESOURCE_PREFIX}-network"
SUBNET_NAME="${RESOURCE_PREFIX}-subnet"
MIG_NAME="${RESOURCE_PREFIX}-mig"
TEMPLATE_NAME="${RESOURCE_PREFIX}-template"
FIREWALL_RULE="${RESOURCE_PREFIX}-allow-internal"
BASE_INSTANCE_NAME="${RESOURCE_PREFIX}-vm"
SSH_FIREWALL_RULE="${RESOURCE_PREFIX}-allow-ssh"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "Dry-run: gcloud config set project ${PROJECT_ID}" >&2
  echo "Dry-run: gcloud config set compute/region ${REGION}" >&2
else
  gcloud config set project "${PROJECT_ID}" >/dev/null
  gcloud config set compute/region "${REGION}" >/dev/null
fi

# Helper that deletes a resource if it exists. Arguments mirror gcloud usage.
delete_if_exists() {
  local description=$1
  shift
  echo "Deleting ${description}..."
  local cmd=("$@" "--quiet")
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "Dry-run: ${cmd[*]}" >&2
    return 0
  fi
  echo "Running: ${cmd[*]}" >&2
  if ! "${cmd[@]}" >/dev/null 2>&1; then
    echo "Skipping ${description}; not found" >&2
  fi
}

# Drain and delete the regional MIG (includes managed instances).
if gcloud compute instance-groups managed describe "${MIG_NAME}" --region "${REGION}" >/dev/null 2>&1; then
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "Dry-run: would resize MIG ${MIG_NAME} to 0 and wait for instances to delete."
  else
    echo "Resizing MIG ${MIG_NAME} to 0..."
    gcloud compute instance-groups managed resize "${MIG_NAME}" --region "${REGION}" --size=0 --quiet
    echo "Waiting for instances to delete..."
    gcloud compute instance-groups managed wait-until "${MIG_NAME}" --region "${REGION}" --stable --quiet || true
  fi
  delete_if_exists "managed instance group ${MIG_NAME}" \
    gcloud compute instance-groups managed delete "${MIG_NAME}" --region "${REGION}"
fi

delete_if_exists "instance template ${TEMPLATE_NAME}" \
  gcloud compute instance-templates delete "${TEMPLATE_NAME}"

delete_if_exists "firewall rule ${FIREWALL_RULE}" \
  gcloud compute firewall-rules delete "${FIREWALL_RULE}"

delete_if_exists "firewall rule ${SSH_FIREWALL_RULE}" \
  gcloud compute firewall-rules delete "${SSH_FIREWALL_RULE}"

delete_if_exists "subnet ${SUBNET_NAME}" \
  gcloud compute networks subnets delete "${SUBNET_NAME}" --region "${REGION}"

delete_if_exists "VPC network ${NETWORK_NAME}" \
  gcloud compute networks delete "${NETWORK_NAME}"

cat <<'NEXT'

Teardown complete. Verify no additional resources (e.g., disks, logs sinks)
remain before closing the project.

NEXT

