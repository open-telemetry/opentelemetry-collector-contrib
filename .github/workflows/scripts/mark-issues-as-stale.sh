#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# This script checks for issues that have been inactive for a certain number
# of days. Any inactive issues have codeowners pinged for labels corresponding
# to a component and are marked as stale. The stale bot will then handle
# the rest of the lifecycle, including removing the stale label and closing
# the issue.
#
# This script is necessary instead of just using the stale action because
# the stale action does not support pinging code owners, and pinging
# code owners after marking an issue as stale will cause the issue to
# have the stale label removed according to all documented behavior
# of the stale action.

set -euo pipefail

if [[ -z ${DAYS_BEFORE_STALE:-} || -z ${DAYS_BEFORE_CLOSE:-} || -z ${STALE_LABEL:-} || -z ${EXEMPT_LABEL:-} ]]; then
    echo "At least one of DAYS_BEFORE_STALE, DAYS_BEFORE_CLOSE, STALE_LABEL, or EXEMPT_LABEL has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")
STALE_MESSAGE="This issue has been inactive for ${DAYS_BEFORE_STALE} days. It will be closed in ${DAYS_BEFORE_CLOSE} days if there is no activity. To ping code owners by adding a component label, see [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments), or if you are unsure of which component this issue relates to, please ping \`@open-telemetry/collector-contrib-triagers\`. If this issue is still relevant, please ping the code owners or leave a comment explaining why it is still relevant. Otherwise, please close it."

# Check for the least recently-updated issues that aren't currently stale.
# If no issues in this list are stale, the repo has no stale issues.
ISSUES=$(gh issue list --limit 100 --search "is:issue is:open -label:\"${STALE_LABEL}\" -label:\"${EXEMPT_LABEL}\" sort:updated-asc" --json number --jq '.[].number')

for ISSUE in ${ISSUES}; do
    OWNER_MENTIONS=''

    UPDATED_AT=$(gh issue view "${ISSUE}" --json updatedAt --jq '.updatedAt')
    UPDATED_UNIX=$(date +%s --date="${UPDATED_AT}")
    NOW=$(date +%s)
    DIFF_DAYS=$(((NOW-UPDATED_UNIX)/(3600*24)))

    if (( DIFF_DAYS < DAYS_BEFORE_STALE )); then
        echo "Issue #${ISSUE} is not stale: it has only been inactive for ${DIFF_DAYS} days and the threshold is ${DAYS_BEFORE_STALE} days. Issues are sorted by updated date in ascending order, so all remaining issues must not be stale. Exiting."
        exit 0
    fi

    LABELS=$(gh issue view "${ISSUE}" --json labels --jq '.labels.[].name')

    for LABEL in ${LABELS}; do
        COMPONENT="${LABEL}"
        OWNERS=$(COMPONENT=${LABEL} bash "${CUR_DIRECTORY}/get-codeowners.sh")

        if [[ -z "${OWNERS}" ]]; then
            continue
        fi

        OWNER_MENTIONS+="- ${COMPONENT}: ${OWNERS}\n"
    done

    if [[ -z "${OWNER_MENTIONS}" ]]; then
        echo "No code owners found. Marking issue #${ISSUE} as stale without pinging code owners."

        gh issue comment "${ISSUE}" -b "${STALE_MESSAGE}"
    else
        printf "Pinging code owners for issue #${ISSUE}:\n${OWNER_MENTIONS}"

        # The GitHub CLI only offers multiline strings through file input.
        printf "${STALE_MESSAGE}\n\nPinging code owners:\n${OWNER_MENTIONS}\nSee [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments) if you do not have permissions to add labels yourself." \
          | gh issue comment "${ISSUE}" -F -
    fi

    # We want to add a label after making a comment for two reasons:
    # 1. If there is some error making a comment, a stale label should not be applied.
    #    We want code owners to be pinged before closing an issue as stale.
    # 2. The stale bot (as of v6) uses the timestamp for when the stale label was
    #    applied to determine when an issue was marked stale. We want to ensure that
    #    was the last activity on the issue, or the stale bot will remove the stale
    #    label if our comment to ping code owners comes too long after the stale
    #    label is applied.
    echo "Marking issue #${ISSUE} as stale."
    gh issue edit "${ISSUE}" --add-label "${STALE_LABEL}"
done

