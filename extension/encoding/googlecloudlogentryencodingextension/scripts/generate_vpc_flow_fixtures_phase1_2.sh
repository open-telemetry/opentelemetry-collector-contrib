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

# Generates VPC flow logs that exercise the Phase 1-2 fields required for
# googlecloudlogentryencodingextension fixtures. Specifically, it targets:
#   * Phase 1 (Round trip + MIG region)
#       - jsonPayload.round_trip_time.median_msec
#       - jsonPayload.src_instance.managed_instance_group.region
#       - jsonPayload.dest_instance.managed_instance_group.region
#   * Phase 2 (Google service annotations)
#       - jsonPayload.src_google_service.{type,service_name,connectivity,private_domain}
#       - jsonPayload.dest_google_service.{type,service_name,connectivity,private_domain}
#
# The script intentionally avoids automatic teardown so resources remain
# available for manual inspection.
#
# Prerequisites:
#   - gcloud CLI v448.0.0+ with beta components
#   - jq
#   - Go 1.21+ (for helper program)
#   - An existing Google Cloud project with billing enabled
#   - API enablement: compute.googleapis.com, logging.googleapis.com
#
# Usage:
#   cp scripts/vpc_flow_fixtures.env.example scripts/vpc_flow_fixtures.env
#   # edit the env file, then:
#   scripts/generate_vpc_flow_fixtures_phase1_2.sh
#
# Alternatively set env vars inline:
#   PROJECT_ID=my-project REGION=us-central1 ZONE=us-central1-b \
#   scripts/generate_vpc_flow_fixtures_phase1_2.sh
#
# Optional knobs:
#   RESOURCE_PREFIX (default: gcp-fixture)

# Prevent accidental use of positional parameters that could be misinterpreted.
if [[ $# -ne 0 ]]; then
  echo "This script does not accept positional arguments." >&2
  exit 1
fi

# Ensure the caller provided the core deployment knobs up front.
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

# Basic tooling validation. The rest of the script relies heavily on gcloud/JQ.
command -v gcloud >/dev/null || {
  echo "gcloud CLI is required" >&2
  exit 1
}

command -v jq >/dev/null || {
  echo "jq is required" >&2
  exit 1
}

echo "Checking gcloud authentication..."
if ! gcloud auth list --format='value(account)' | grep -q "@"; then
  echo "No active gcloud account. Run 'gcloud auth login' first." >&2
  exit 1
fi

echo "Configuring defaults for project ${PROJECT_ID}, region ${REGION}, zone ${ZONE}"
gcloud config set project "${PROJECT_ID}" >/dev/null
gcloud config set compute/region "${REGION}" >/dev/null
gcloud config set compute/zone "${ZONE}" >/dev/null

# Derive resource names from the shared prefix.
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

echo "Ensuring VPC network ${NETWORK_NAME} exists..."
if ! gcloud compute networks describe "${NETWORK_NAME}" --format='value(name)' >/dev/null 2>&1; then
  # Dedicated network keeps test traffic isolated.
  gcloud compute networks create "${NETWORK_NAME}" \
    --subnet-mode=custom
fi

echo "Ensuring subnet ${SUBNET_NAME} exists with flow logs enabled..."
if ! gcloud compute networks subnets describe "${SUBNET_NAME}" --region "${REGION}" --format='value(name)' >/dev/null 2>&1; then
  # New subnet with flow logs and aggressive sampling to guarantee log output.
  gcloud compute networks subnets create "${SUBNET_NAME}" \
    --network="${NETWORK_NAME}" \
    --region="${REGION}" \
    --range=10.10.0.0/20 \
    --enable-flow-logs \
    --logging-aggregation-interval=interval-5-min \
    --logging-flow-sampling=1.0 \
    --logging-metadata=include-all
else
  # If the subnet already exists, make sure logging knobs are correct.
  gcloud compute networks subnets update "${SUBNET_NAME}" \
    --region="${REGION}" \
    --enable-flow-logs \
    --logging-aggregation-interval=interval-5-min \
    --logging-flow-sampling=1.0 \
    --logging-metadata=include-all
fi

echo "Ensuring internal firewall rules allow intra-subnet (east-west) traffic..."
gcloud compute firewall-rules describe "${FIREWALL_RULE}" >/dev/null 2>&1 || {
  # Allow VM-to-VM communication within the subnet so ping/iperf can function.
  gcloud compute firewall-rules create "${FIREWALL_RULE}" \
    --network="${NETWORK_NAME}" \
    --allow=tcp,udp,icmp \
    --direction=INGRESS \
    --source-ranges=10.10.0.0/20
}

SSH_FIREWALL_RULE="${RESOURCE_PREFIX}-allow-ssh"
gcloud compute firewall-rules describe "${SSH_FIREWALL_RULE}" >/dev/null 2>&1 || {
  # Open SSH to the internet for the short-lived fixture hosts.
  gcloud compute firewall-rules create "${SSH_FIREWALL_RULE}" \
    --network="${NETWORK_NAME}" \
    --allow=tcp:22 \
    --direction=INGRESS \
    --source-ranges=0.0.0.0/0 \
    --description="Temporary SSH access for VPC flow fixture instances"
}

echo "Creating instance template ${TEMPLATE_NAME} (regional MIG)..."
gcloud compute instance-templates describe "${TEMPLATE_NAME}" >/dev/null 2>&1 || {
  # Template wires in the startup script so every instance begins generating traffic immediately.
  gcloud compute instance-templates create "${TEMPLATE_NAME}" \
    --machine-type=e2-standard-2 \
    --network="${NETWORK_NAME}" \
    --subnet="${SUBNET_NAME}" \
    --metadata-from-file startup-script="${SCRIPT_DIR}/guest/traffic_startup.sh" \
    --tags=vpc-fixture
}

echo "Creating regional managed instance group ${MIG_NAME}..."
if ! gcloud compute instance-groups managed describe "${MIG_NAME}" --region "${REGION}" >/dev/null 2>&1; then
  # Regional scope is required to surface managed_instance_group.region in flow logs.
  gcloud compute instance-groups managed create "${MIG_NAME}" \
    --region="${REGION}" \
    --base-instance-name="${BASE_INSTANCE_NAME}" \
    --size=2 \
    --template="${TEMPLATE_NAME}"
else
  gcloud compute instance-groups managed resize "${MIG_NAME}" --region "${REGION}" --size=2
  echo "Updating MIG ${MIG_NAME} to use template ${TEMPLATE_NAME}..."
  gcloud compute instance-groups managed set-instance-template "${MIG_NAME}" \
    --region "${REGION}" \
    --template "${TEMPLATE_NAME}"
  echo "Restarting instances to apply updated startup script..."
  gcloud compute instance-groups managed update-instances "${MIG_NAME}" \
    --region "${REGION}" \
    --all-instances \
    --most-disruptive-allowed-action=restart \
    --minimal-action=restart
fi

echo "Fetching instance IPs to seed peer metadata..."
# Instance discovery and metadata assignment is handled by a Go helper for portability.
if ! helper_output=$(go run "${SCRIPT_DIR}/internal/fixtures/instance_helper.go" --mig-name "${MIG_NAME}" --region "${REGION}"); then
  echo "Failed to gather instance information. Ensure Go and gcloud are installed." >&2
  exit 1
fi

if ! jq -e '.instances | length >= 2' >/dev/null 2>&1 <<<"${helper_output}"; then
  echo "Expected at least two instances in the managed group; received: ${helper_output}" >&2
  exit 1
fi

while IFS=$'\t' read -r name zone peer_ip; do
  [[ -z "${name}" ]] && continue
  gcloud compute instances add-metadata "${name}" \
    --metadata="peer-ip=${peer_ip}" \
    --zone="${zone}"
  gcloud compute ssh "${name}" \
    --command='sudo /usr/local/bin/generate_traffic.sh' \
    --zone="${zone}" || true
done < <(jq -r '.instances[] | [.name, .zone, .peer_ip] | @tsv' <<<"${helper_output}")

cat <<'NEXT'

Next steps:
  - Allow ~10 minutes for VPC flow logs to ingest (aggregation interval 5 min).
  - Use scripts/export_vpc_flow_logs_phase1_2.sh to capture relevant entries.
  - Remember to manually clean up resources when finished to avoid charges.

NEXT

