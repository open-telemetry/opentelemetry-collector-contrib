#!/usr/bin/env bash

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

# Tears down the infrastructure created by generate_vpc_flow_fixtures.sh.
# The script now relies on Terraform to calculate and destroy the resource graph.

DRY_RUN="true"
for arg in "$@"; do
  case "${arg}" in
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
      echo "Unknown argument: ${arg}" >&2
      exit 1
      ;;
  esac
done

echo "Terraform teardown running with dry-run=${DRY_RUN}" 

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
command -v terraform >/dev/null || { echo "terraform is required" >&2; exit 1; }

RESOURCE_PREFIX=${RESOURCE_PREFIX:-gcp-fixture}

# Export Terraform variables via TF_VAR_* environment variables
export TF_VAR_project_id="${PROJECT_ID}"
export TF_VAR_region="${REGION}"
export TF_VAR_zone="${ZONE}"
export TF_VAR_resource_prefix="${RESOURCE_PREFIX}"

cd "${TERRAFORM_DIR}"

echo "Initializing Terraform (safe to rerun)..."
terraform init -upgrade >/dev/null

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "Running terraform plan -destroy (dry-run)..."
  terraform plan -destroy
else
  echo "Running terraform destroy..."
  terraform destroy
fi

echo "Terraform teardown complete."

