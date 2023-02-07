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

set -euo pipefail

if [[  -z "${REPO:-}" || -z "${AUTHOR:-}" || -z "${COMPONENT:-}" || -z "${PR:-}" ]]; then
    echo "At least one of REPO, AUTHOR, COMPONENT, or PR has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")

main() {
    OWNERS=$(COMPONENT="${COMPONENT}" bash "${CUR_DIRECTORY}/get-codeowners.sh")
    REVIEWERS=""

    if [[ -z "${OWNERS}" ]]; then
        exit 0
    fi

    for OWNER in ${OWNERS}; do
        if [[ "${OWNER}" = "@${AUTHOR}" ]]; then
            continue
        fi
    
        if [[ -n "${REVIEWERS}" ]]; then
            REVIEWERS+=","
        fi
        REVIEWERS+=$(echo "${OWNER}" | sed -E 's/@(.+)/"\1"/')
    done

    # We have to use the GitHub API directly due to an issue with how the CLI
    # handles PR updates that causes it require access to organization teams,
    # and the GitHub token doesn't provide that permission.
    # For more: https://github.com/cli/cli/issues/4844
    #
    # The GitHub API validates that authors are not requested to review, but
    # accepts duplicate logins and logins that are already reviewers.
    echo "Requesting review from code owners: ${REVIEWERS}"
    curl \
        -X POST \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${GITHUB_TOKEN}" \
        "https://api.github.com/repos/${REPO}/pulls/${PR}/requested_reviewers" \
        -d "{\"reviewers\":[${REVIEWERS}]}" \
        | jq ".message" \
        || echo "jq was unable to parse GitHub's response"
}

# We don't want this workflow to ever fail and block a PR,
# so ensure all errors are caught.
main || echo "Failed to run $0"
