#!/usr/bin/env bash
#
#   Copyright The OpenTelemetry Authors.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
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

CUR_DIRECTORY=$(dirname "$0")
COMPONENTS=$(sh "${CUR_DIRECTORY}/get-components.sh")

for COMPONENT in ${COMPONENTS}; do
    TYPE=$(echo "${COMPONENT}" | cut -f1 -d '/' )
    LABEL_NAME=$(echo "${COMPONENT}" | sed -E 's%^(.+)/(.+)\1%\1/\2%')

    if (( "${#LABEL_NAME}" > 50 )); then
        echo "'${LABEL_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
        continue
    fi

    gh label create "${LABEL_NAME}" -c "${COLORS["${TYPE}"]:-${FALLBACK_COLOR}}" --force
done

