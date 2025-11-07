#!/usr/bin/env bash
set -euo pipefail

apt-get update
apt-get install -y iperf3 curl jq

cat <<'EOF' >/usr/local/bin/generate_traffic.sh
#!/bin/bash
set -euo pipefail

LOG_DIR=/var/log/vpc-fixture
mkdir -p "${LOG_DIR}"
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
LOG_FILE="${LOG_DIR}/traffic_${TIMESTAMP}.log"
echo "writing traffic log to ${LOG_FILE}"
exec > >(tee -a "${LOG_FILE}") 2>&1

other_instance=$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/peer-ip") || true
if [[ -n "${other_instance}" ]]; then
  ping -c 40 "${other_instance}" || true
  iperf3 -c "${other_instance}" -t 45 || true
fi

echo "anonymous GET storage.googleapis.com"
curl -sv https://storage.googleapis.com/ -o /tmp/storage_index.html || true
echo "anonymous GET discovery APIs"
curl -sv https://www.googleapis.com/discovery/v1/apis -o /tmp/discovery.json || true

project_id=$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/project/project-id")
access_token=$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | jq -r '.access_token') || true
if [[ -n "${access_token}" && "${access_token}" != "null" ]]; then
  echo "auth GET storage bucket list"
  curl -sv -H "Authorization: Bearer ${access_token}" \
    "https://storage.googleapis.com/storage/v1/b?project=${project_id}&maxResults=1" -o /tmp/storage_buckets.json || true
  echo "auth GET compute regions"
  curl -sv -H "Authorization: Bearer ${access_token}" \
    "https://compute.googleapis.com/compute/v1/projects/${project_id}/regions" -o /tmp/compute_regions.json || true
fi
EOF

chmod +x /usr/local/bin/generate_traffic.sh

