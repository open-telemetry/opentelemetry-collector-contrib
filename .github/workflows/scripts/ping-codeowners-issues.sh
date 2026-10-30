#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# Pings code owners on a labeled issue or discussion.
# Usage: ping-codeowners-issues.sh <issue|discussion>

set -euo pipefail

TARGET_TYPE="${1:-}"
if [[ "${TARGET_TYPE}" != "issue" && "${TARGET_TYPE}" != "discussion" ]]; then
    echo "Usage: $0 <issue|discussion>"
    exit 1
fi

if [[ "${TARGET_TYPE}" = "issue" ]]; then
    if [[ -z "${COMPONENT:-}" || -z "${ISSUE:-}" ]]; then
        echo "Either COMPONENT or ISSUE has not been set, please ensure both are set."
        exit 0
    fi
else
    if [[ -z "${COMPONENT:-}" || -z "${DISCUSSION_ID:-}" ]]; then
        echo "Either COMPONENT or DISCUSSION_ID has not been set, please ensure both are set."
        exit 0
    fi
fi

CUR_DIRECTORY=$(dirname "$0")
COMPONENT=$(COMPONENT="${COMPONENT}" "${CUR_DIRECTORY}/get-label-from-component.sh" || true)
# Some labels are unrelated to components. These labels do not have code owners,
# e.g "os:windows", "priority:p1", and "chore"
if [[ -z "${COMPONENT}" ]]; then
    exit 0
fi

OWNERS=$(COMPONENT="${COMPONENT}" bash "${CUR_DIRECTORY}/get-codeowners.sh")

if [[ -z "${OWNERS}" ]]; then
    exit 0
fi

BODY="Pinging code owners for ${COMPONENT}: ${OWNERS}. See [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments) if you do not have permissions to add labels yourself. For example, comment '/label priority:p2 -needs-triage' to set the priority and remove the needs-triage label."

if [[ "${TARGET_TYPE}" = "issue" ]]; then
    gh issue comment "${ISSUE}" --body "${BODY}"
else
    jq -n --arg id "${DISCUSSION_ID}" --arg body "${BODY}" \
        '{query: "mutation($id: ID!, $body: String!) { addDiscussionComment(input: {discussionId: $id, body: $body}) { clientMutationId } }", variables: {id: $id, body: $body}}' |
        gh api graphql --input -
fi
