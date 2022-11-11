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
# Adds code owners without write access as reviewers on a PR. Note that
# the code owners must still be a member of the `open-telemetry`
# organization.
#
# Note that since this script is considered a requirement for PRs,
# it should never fail.

set -euo pipefail

if [[ -z "${REPO:-}" || -z "${PR:-}" ]]; then
    echo "One or more of REPO and PR have not been set, please ensure each is set."
    exit 0
fi

main () {
    CUR_DIRECTORY=$(dirname "$0")
    JSON=$(gh pr view "${PR}" --repo evan-bradley/opentelemetry-collector-contrib --json "files,author")
    AUTHOR=$(printf "${JSON}"| jq '.author.login')
    FILES=$(printf "${JSON}"| jq -r '.files[].path')
    COMPONENTS=$(grep -oE '^[a-z]+/[a-z/]+ ' < .github/CODEOWNERS)
    REVIEWERS=""
    LABELS=""
    declare -A PROCESSED_COMPONENTS

    for COMPONENT in ${COMPONENTS}; do
        # Files will be in alphabetical order and there are many files to
        # a component, so loop through files in an inner loop. This allows
        # us to remove all files for a component from the list so they
        # won't be checked against the remaining components in the components
        # list. This provides a meaningful speedup in practice.
        for FILE in ${FILES}; do
            MATCH=$(echo "${FILE}" | grep -E "^${COMPONENT}" || true)

            if [[ -z "${MATCH}" ]]; then
                continue
            fi

            # If we match a file with a component we don't need to process the file again.
            FILES=$(printf "${FILES}" | grep -v "${FILE}")

            if [[ -v PROCESSED_COMPONENTS["${COMPONENT}"] ]]; then
                continue
            fi

            PROCESSED_COMPONENTS["${COMPONENT}"]=true

            OWNERS=$(COMPONENT="${COMPONENT}" bash "${CUR_DIRECTORY}/get-codeowners.sh")

            if [[ -n "${OWNERS}" ]]; then
                if [[ -n "${REVIEWERS}" ]]; then
                    REVIEWERS+=","
                fi
                REVIEWERS+="$(echo "${OWNERS}" | sed -E 's/@([A-Za-z0-9_-]+)( |$)/"\1"\2/g' | sed -E "s/${AUTHOR} ?//g" | sed 's/ /,/g')"
            fi

            # Convert the CODEOWNERS entry to a label
            COMPONENT_CLEAN=$(echo "${COMPONENT}" | sed -E 's%/$%%')
            TYPE=$(echo "${COMPONENT_CLEAN}" | cut -f1 -d '/' )
            NAME=$(echo "${COMPONENT_CLEAN}" | cut -f2- -d '/' | sed -E "s%${TYPE}\$%%")

            if [[ -n "${LABELS}" ]]; then
                LABELS+=","
            fi
            LABELS+="${TYPE}/${NAME}"
        done
    done

    gh pr edit "${PR}" --add-label "${LABELS}" || echo "Failed to add labels to #${PR}"

    # Note that adding the labels above will not trigger any other workflows to
    # add code owners, so we have to do it here.
    #
    # We have to use the GitHub API directly due to an issue with how the CLI
    # handles PR updates that causes it require access to organization teams,
    # and the GitHub token doesn't provide that permission.
    # For more: https://github.com/cli/cli/issues/4844
    #
    # The GitHub API validates that authors are not requested to review, but
    # accepts duplicate logins and logins that are already reviewers.
    curl \
        --fail \
        -X POST \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${GITHUB_TOKEN}" \
        "https://api.github.com/repos/${REPO}/pulls/${PR}/requested_reviewers" \
        -d "{\"reviewers\":[${REVIEWERS}]}" \
        | jq ".message" \
        || echo "Failed to add reviewers to #${PR}"
}

main || echo "Failed to run $0"
