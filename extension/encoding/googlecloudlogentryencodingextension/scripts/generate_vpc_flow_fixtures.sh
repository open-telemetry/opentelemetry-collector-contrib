#!/usr/bin/env bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"
ENV_FILE=${ENV_FILE:-"${SCRIPT_DIR}/vpc_flow_fixtures.env"}

if [[ -f "${ENV_FILE}" ]]; then
  echo "Loading environment from ${ENV_FILE}"
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

# Generates VPC flow logs for googlecloudlogentryencodingextension fixtures.
# Terraform provisions the infrastructure; this script feeds variables,
# invokes Terraform, then runs the Go helper to decorate instances and start
# traffic generation.

if [[ $# -ne 0 ]]; then
  echo "This script does not accept positional arguments." >&2
  exit 1
fi

required_env=(PROJECT_ID REGION ZONE)
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

command -v gcloud >/dev/null || { echo "gcloud is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }
command -v terraform >/dev/null || { echo "terraform is required" >&2; exit 1; }
command -v go >/dev/null || { echo "go is required" >&2; exit 1; }

echo "Checking gcloud authentication..."
if ! gcloud auth list --format='value(account)' | grep -q "@"; then
  echo "No active gcloud account. Run 'gcloud auth login' first." >&2
  exit 1
fi

RESOURCE_PREFIX=${RESOURCE_PREFIX:-gcp-fixture}

# Export Terraform variables via TF_VAR_* environment variables
export TF_VAR_project_id="${PROJECT_ID}"
export TF_VAR_region="${REGION}"
export TF_VAR_zone="${ZONE}"
export TF_VAR_resource_prefix="${RESOURCE_PREFIX}"

cd "${TERRAFORM_DIR}"

echo "Initializing Terraform (safe to rerun)..."
terraform init -upgrade >/dev/null

echo "Applying Terraform infrastructure..."
terraform apply


echo "Collecting Terraform outputs..."
tf_output=$(terraform output -json)
MIG_NAME=$(jq -r '.mig_name.value' <<<"${tf_output}")
REGION_OUTPUT=$(jq -r '.region.value' <<<"${tf_output}")
SUBNET_NAME=$(jq -r '.subnet_name.value' <<<"${tf_output}")

if [[ -z "${MIG_NAME}" || "${MIG_NAME}" == "null" ]]; then
  echo "Failed to retrieve managed instance group name from Terraform outputs." >&2
  exit 1
fi

echo "Managed instance group: ${MIG_NAME}"
echo "Subnet: ${SUBNET_NAME}"

echo "Generating traffic from instances..."
if ! go run "${SCRIPT_DIR}/internal/fixtures/mig_traffic_runner.go" \
  --mig-name "${MIG_NAME}" \
  --region "${REGION_OUTPUT}" \
  --project-id "${PROJECT_ID}" \
  --generate-traffic; then
  echo "Traffic generation failed." >&2
  exit 1
fi

cat <<'NEXT'

Next steps:
  - Allow ~10 minutes for VPC flow logs to ingest (aggregation interval 5 min).
  - Use scripts/export_vpc_flow_logs.sh to capture relevant entries.
  - Remember to manually clean up resources when finished to avoid charges.

NEXT

