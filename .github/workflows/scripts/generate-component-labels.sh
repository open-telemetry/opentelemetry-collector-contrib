#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# Note that there is a 50-character limit on labels, so some components may
# not have a corresponding label.

set -euo pipefail

declare -A COLORS

COLORS["cmd"]="#483C32"
COLORS["confmap"]="#6666FF"
COLORS["examples"]="#1A8CFF"
COLORS["exporter"]="#50C878"
COLORS["extension"]="#FF794D"
COLORS["internal"]="#B30059"
COLORS["pkg"]="#F9DE22"
COLORS["processor"]="#800080"
COLORS["receiver"]="#E91B7B"
COLORS["testbed"]="#336600"

FALLBACK_COLOR="#999966"

COMPONENTS=$(awk '/^[^#]/ { print $2 }' .github/component_labels.txt)

for LABEL_NAME in ${COMPONENTS}; do
    TYPE=$(echo "${LABEL_NAME}" | cut -f1 -d '/' )
    
    if (( "${#LABEL_NAME}" > 50 )); then
        echo "'${LABEL_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
        continue
    fi

    gh label create "${LABEL_NAME}" -c "${COLORS["${TYPE}"]:-${FALLBACK_COLOR}}" --force
done

