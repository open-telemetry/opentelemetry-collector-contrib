#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# Discussions equivalent of add-labels.sh. The GitHub CLI and REST API do not
# expose labels on discussions, so labels are added/removed through the
# GraphQL addLabelsToLabelable/removeLabelsFromLabelable mutations instead of
# `gh issue edit`. See https://docs.github.com/en/graphql/guides/using-the-graphql-api-for-discussions.

set -euo pipefail

if [[ -z "${DISCUSSION_ID:-}" || -z "${COMMENT:-}" || -z "${SENDER:-}" ]]; then
    echo "At least one of DISCUSSION_ID, COMMENT, or SENDER has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")

if [[ ${COMMENT:0:6} != "/label" ]]; then
    echo "Comment is not a label comment, exiting."
    exit 0
fi

source "${CUR_DIRECTORY}/common-labels.sh"

get_label_id() {
    local NAME_ENCODED
    NAME_ENCODED=$(jq -rn --arg n "$1" '$n|@uri')
    # On a 404 (label doesn't exist), gh prints the raw error body instead of
    # applying --jq, so only trust the output when the call actually succeeds.
    gh api "repos/open-telemetry/opentelemetry-collector-contrib/labels/${NAME_ENCODED}" --jq '.node_id' 2>/dev/null
}

mutate_label() {
    local MUTATION_NAME="$1"
    local LABEL_ID="$2"
    jq -n \
        --arg mutation "$(printf 'mutation($id: ID!, $labelIds: [ID!]!) { %s(input: {labelableId: $id, labelIds: $labelIds}) { clientMutationId } }' "${MUTATION_NAME}")" \
        --arg id "${DISCUSSION_ID}" \
        --arg labelId "${LABEL_ID}" \
        '{query: $mutation, variables: {id: $id, labelIds: [$labelId]}}' |
        gh api graphql --input -
}

LABELS=$(echo "${COMMENT}" | sed -E 's%^/label%%')

for LABEL_REQ in ${LABELS}; do
    LABEL=$(echo "${LABEL_REQ}" | sed -E s/^[+-]?//)
    # Trim newlines from label that would cause matching to fail
    LABEL=$(echo "${LABEL}" | tr -d '\n')

    SHOULD_ADD=true
    if [[ "${LABEL_REQ:0:1}" = "-" ]]; then
        SHOULD_ADD=false
    fi

    FULL_LABEL=""
    IS_COMPONENT=false
    if [[ -v COMMON_LABELS["${LABEL}"] ]]; then
        FULL_LABEL="${COMMON_LABELS["${LABEL}"]}"
    else
        FULL_LABEL=$(awk -v path="${LABEL}" '$1 == path || $2 == path {print $2}' .github/component_labels.txt)
        IS_COMPONENT=true
    fi

    if [[ -z "${FULL_LABEL}" ]]; then
        echo "\"${LABEL}\" doesn't correspond to a known label or component, skipping."
        continue
    fi

    if ! LABEL_ID=$(get_label_id "${FULL_LABEL}"); then
        LABEL_ID=""
    fi

    if [[ -z "${LABEL_ID}" ]]; then
        echo "Could not resolve label \"${FULL_LABEL}\" to a GraphQL node ID, skipping."
        continue
    fi

    if [[ ${SHOULD_ADD} = true ]]; then
        mutate_label "addLabelsToLabelable" "${LABEL_ID}"

        if [[ ${IS_COMPONENT} = true ]]; then
            # Labels added by a GitHub Actions workflow don't trigger other workflows
            # by design, so we have to manually ping code owners here.
            COMPONENT="${LABEL}" DISCUSSION_ID="${DISCUSSION_ID}" bash "${CUR_DIRECTORY}/ping-codeowners-issues.sh" discussion
        fi
    else
        mutate_label "removeLabelsFromLabelable" "${LABEL_ID}"
    fi
done
