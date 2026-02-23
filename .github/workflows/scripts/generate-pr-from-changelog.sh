#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# Generates a PR title and body from changelog entry YAML files
# when the PR title contains "as per changelog" (case-insensitive).
#
# Required environment variables:
#   REPO    - The GitHub repository (e.g. "open-telemetry/opentelemetry-collector-contrib")
#   PR      - The PR number
#   PR_HEAD - The HEAD SHA of the PR

set -euo pipefail

if [[ -z "${REPO:-}" || -z "${PR:-}" || -z "${PR_HEAD:-}" ]]; then
    echo "One or more of REPO, PR, and PR_HEAD have not been set, please ensure each is set."
    exit 0
fi

# Map change_type values to human-readable prefixes
change_type_label() {
    case "$1" in
        breaking)       echo "breaking change" ;;
        deprecation)    echo "deprecation" ;;
        new_component)  echo "new component" ;;
        enhancement)    echo "enhancement" ;;
        bug_fix)        echo "bug fix" ;;
        *)              echo "$1" ;;
    esac
}

main() {
    # Find changelog YAML files added in this PR (excluding TEMPLATE.yaml, config.yaml)
    ADDED_FILES=$(git diff --diff-filter=A --name-only "$(git merge-base origin/main "${PR_HEAD}")" "${PR_HEAD}" ./.chloggen/ \
        | grep -E '\.yaml$' \
        | grep -v 'TEMPLATE\.yaml' \
        | grep -v 'config\.yaml' \
        || true)

    if [[ -z "${ADDED_FILES}" ]]; then
        echo "No changelog entry YAML files found in this PR. Cannot generate PR info from changelog."
        exit 0
    fi

    echo "Found changelog entries:"
    echo "${ADDED_FILES}"

    # Collect information from all changelog entries
    COMPONENTS=()
    NOTES=()
    CHANGE_TYPES=()
    ALL_ISSUES=()
    SUBTEXTS=()

    for FILE in ${ADDED_FILES}; do
        if [[ ! -f "${FILE}" ]]; then
            echo "Warning: File ${FILE} not found, skipping."
            continue
        fi

        # Parse YAML fields using grep + sed (avoids requiring yq as a dependency)
        # Extract change_type
        CHANGE_TYPE=$(grep -E '^change_type:' "${FILE}" | head -1 | sed "s/^change_type:[[:space:]]*//" | sed "s/['\"]//g" | xargs)

        # Extract component
        COMPONENT=$(grep -E '^component:' "${FILE}" | head -1 | sed "s/^component:[[:space:]]*//" | sed "s/['\"]//g" | xargs)

        # Extract note
        NOTE=$(grep -E '^note:' "${FILE}" | head -1 | sed "s/^note:[[:space:]]*//" | sed "s/['\"]//g" | xargs)

        # Extract issues (format: issues: [1234] or issues: [1234, 5678])
        ISSUES=$(grep -E '^issues:' "${FILE}" | head -1 | sed "s/^issues:[[:space:]]*//" | tr -d '[]' | xargs)

        # Extract subtext (can be multiline with pipe |)
        SUBTEXT_LINE=$(grep -n '^subtext:' "${FILE}" | head -1)
        SUBTEXT=""
        if [[ -n "${SUBTEXT_LINE}" ]]; then
            LINE_NUM=$(echo "${SUBTEXT_LINE}" | cut -d: -f1)
            SUBTEXT_VALUE=$(echo "${SUBTEXT_LINE}" | sed "s/^[0-9]*:subtext:[[:space:]]*//" | xargs)

            if [[ "${SUBTEXT_VALUE}" == "|" ]]; then
                # Multiline subtext: read indented lines after the pipe
                SUBTEXT=$(awk -v start="${LINE_NUM}" '
                    NR > start {
                        if (/^[[:space:]]+[^#]/) {
                            sub(/^[[:space:]]{2}/, ""); print
                        } else if (/^[[:space:]]*$/) {
                            print ""
                        } else {
                            exit
                        }
                    }
                ' "${FILE}")
            elif [[ -n "${SUBTEXT_VALUE}" ]]; then
                # shellcheck disable=SC2001
                SUBTEXT=$(echo "${SUBTEXT_VALUE}" | sed "s/['\"]//g")
            fi
        fi

        if [[ -n "${COMPONENT}" ]]; then
            COMPONENTS+=("${COMPONENT}")
        fi
        if [[ -n "${NOTE}" ]]; then
            NOTES+=("${NOTE}")
        fi
        if [[ -n "${CHANGE_TYPE}" ]]; then
            CHANGE_TYPES+=("${CHANGE_TYPE}")
        fi
        if [[ -n "${ISSUES}" ]]; then
            # Split comma-separated issues
            IFS=',' read -ra ISSUE_ARRAY <<< "${ISSUES}"
            for ISSUE in "${ISSUE_ARRAY[@]}"; do
                ISSUE=$(echo "${ISSUE}" | xargs)
                if [[ -n "${ISSUE}" ]]; then
                    ALL_ISSUES+=("${ISSUE}")
                fi
            done
        fi
        if [[ -n "${SUBTEXT}" ]]; then
            SUBTEXTS+=("${SUBTEXT}")
        fi
    done

    if [[ ${#NOTES[@]} -eq 0 ]]; then
        echo "No valid changelog notes found. Cannot generate PR info."
        exit 0
    fi

    # --- Generate PR Title ---
    # Format: [component] note (for single entry)
    # Format: [component1, component2] first note (+N more) (for multiple entries)

    # Deduplicate components
    mapfile -t UNIQUE_COMPONENTS < <(echo "${COMPONENTS[@]}" | tr ' ' '\n' | sort -u)
    COMPONENT_PREFIX=""
    if [[ ${#UNIQUE_COMPONENTS[@]} -gt 0 ]]; then
        COMPONENT_PREFIX="[$(IFS=', '; echo "${UNIQUE_COMPONENTS[*]}")] "
    fi

    if [[ ${#NOTES[@]} -eq 1 ]]; then
        NEW_TITLE="${COMPONENT_PREFIX}${NOTES[0]}"
    else
        # Multiple entries: use the first note, mentioning there are more
        REMAINING=$(( ${#NOTES[@]} - 1 ))
        NEW_TITLE="${COMPONENT_PREFIX}${NOTES[0]} (+${REMAINING} more)"
    fi

    # Truncate title to 256 characters (GitHub limit)
    if [[ ${#NEW_TITLE} -gt 256 ]]; then
        NEW_TITLE="${NEW_TITLE:0:253}..."
    fi

    echo ""
    echo "Generated PR title: ${NEW_TITLE}"

    # --- Generate PR Body ---
    BODY="#### Description"$'\n'

    for i in "${!NOTES[@]}"; do
        CHANGE_LABEL=""
        if [[ -n "${CHANGE_TYPES[$i]:-}" ]]; then
            CHANGE_LABEL=$(change_type_label "${CHANGE_TYPES[$i]}")
        fi

        if [[ ${#NOTES[@]} -eq 1 ]]; then
            BODY+=$'\n'"**${CHANGE_LABEL^}**: ${NOTES[$i]}"$'\n'
        else
            BODY+=$'\n'"- **${CHANGE_LABEL^}** (${COMPONENTS[$i]:-unknown}): ${NOTES[$i]}"$'\n'
        fi
    done

    # Add subtext if present
    if [[ ${#SUBTEXTS[@]} -gt 0 ]]; then
        BODY+=$'\n'"**Details:**"$'\n'
        for SUBTEXT in "${SUBTEXTS[@]}"; do
            if [[ -n "${SUBTEXT}" ]]; then
                BODY+=$'\n'"${SUBTEXT}"$'\n'
            fi
        done
    fi

    # Add tracking issues
    BODY+=$'\n'"#### Link to tracking issue"$'\n'
    if [[ ${#ALL_ISSUES[@]} -gt 0 ]]; then
        mapfile -t UNIQUE_ISSUES < <(echo "${ALL_ISSUES[@]}" | tr ' ' '\n' | sort -un)
        for ISSUE in "${UNIQUE_ISSUES[@]}"; do
            BODY+="Fixes #${ISSUE}"$'\n'
        done
    else
        BODY+="Fixes "$'\n'
    fi

    BODY+=$'\n'"#### Testing"$'\n\n'
    BODY+="#### Documentation"$'\n\n'

    BODY+="---"$'\n'
    BODY+="*This PR description was auto-generated from the changelog entry.*"$'\n'

    echo ""
    echo "Generated PR body:"
    echo "${BODY}"

    # --- Update the PR ---
    echo ""
    echo "Updating PR #${PR}..."

    gh pr edit "${PR}" --title "${NEW_TITLE}" --body "${BODY}"

    echo "PR #${PR} updated successfully."
}

# We don't want this workflow to ever fail and block a PR,
# so ensure all errors are caught.
main || echo "Failed to run $0"
