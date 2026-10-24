#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
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

source "${CUR_DIRECTORY}/common-labels.sh"

LABELS=$(echo "${COMMENT}" | sed -E 's%^/label%%')

for LABEL_REQ in ${LABELS}; do
    LABEL=$(echo "${LABEL_REQ}" | sed -E s/^[+-]?//)
    # Trim newlines from label that would cause matching to fail
    LABEL=$(echo "${LABEL}" | tr -d '\n')

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

    SHORT_LABEL=$(awk -v path="${LABEL}" '$1 == path || $2 == path {print $2}' .github/component_labels.txt)

    if [[  -z "${SHORT_LABEL}" ]]; then
        echo "\"${SHORT_LABEL}\" doesn't correspond to a component, skipping."
        continue
    fi

    if [[ ${SHOULD_ADD} = true ]]; then
        gh issue edit "${ISSUE}" --add-label "${SHORT_LABEL}"

        # Labels added by a GitHub Actions workflow don't trigger other workflows
        # by design, so we have to manually ping code owners here.
        COMPONENT="${LABEL}" ISSUE=${ISSUE} SENDER="${SENDER}" bash "${CUR_DIRECTORY}/ping-codeowners-issues.sh"
    else
        gh issue edit "${ISSUE}" --remove-label "${LABEL}"
    fi
done


