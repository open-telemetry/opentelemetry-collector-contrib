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
#

set -euo pipefail

if [[ -z "${ISSUE:-}" || -z "${COMMENT:-}" || -z "${SENDER:-}" ]]; then
    echo "At least one of ISSUE, COMMENT, or SENDER has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")

if [[ ${COMMENT:0:6} != "/label" ]]; then
    echo "Comment is not a label comment, exiting."
    exit 0
fi

declare -A COMMON_LABELS
COMMON_LABELS["good-first-issue"]="good first issue"
COMMON_LABELS["help-wanted"]="help wanted"
COMMON_LABELS["needs-discussion"]="needs discussion"
COMMON_LABELS["needs-triage"]="needs triage"
COMMON_LABELS["waiting-for-author"]="waiting for author"

LABELS=$(echo "${COMMENT}" | sed -E 's%^/label%%')

for LABEL_REQ in ${LABELS}; do
    LABEL=$(echo "${LABEL_REQ}" | sed -E s/^[+-]?//)
    SHOULD_ADD=true

    if [[ "${LABEL_REQ:0:1}" = "-" ]]; then
        SHOULD_ADD=false
    fi

    if [[ -v COMMON_LABELS["${LABEL}"] ]]; then
        if [[ ${SHOULD_ADD} = true ]]; then
            gh issue edit "${ISSUE}" --add-label "${COMMON_LABELS["${LABEL}"]}"
        else
            gh issue edit "${ISSUE}" --remove-label "${COMMON_LABELS["${LABEL}"]}"
        fi
        continue
    fi

    # Grep exits with status code 1 if there are no matches,
    # so we manually set RESULT to 0 if nothing is found.
    RESULT=$(grep -c "${LABEL}" .github/CODEOWNERS || true)

    if [[ ${RESULT} = 0 ]]; then
        echo "\"${LABEL}\" doesn't correspond to a component, skipping."
        continue
    fi

    if [[ ${SHOULD_ADD} = true ]]; then
        gh issue edit "${ISSUE}" --add-label "${LABEL}"

        # Labels added by a GitHub Actions workflow don't trigger other workflows
        # by design, so we have to manually ping code owners here.
        COMPONENT="${LABEL}" ISSUE=${ISSUE} SENDER="${SENDER}" bash "${CUR_DIRECTORY}/ping-codeowners.sh"
    else
        gh issue edit "${ISSUE}" --remove-label "${LABEL}"
    fi
done


