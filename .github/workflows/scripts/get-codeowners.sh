#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# This script checks the GitHub CODEOWNERS file for any code owners
# of contrib components and returns a string of the code owners if it
# finds them.

set -euo pipefail

get_component_type() {
  echo "${COMPONENT}" | cut -f 1 -d '/'
}

get_codeowners() {
  echo "$((grep -m 1 "^${1}/\s" .github/CODEOWNERS || true) | \
        sed 's/   */ /g' | \
        cut -f3- -d ' ')"
}

if [[ -z "${COMPONENT:-}" ]]; then
    echo "COMPONENT has not been set, please ensure it is set."
    exit 1
fi

# grep exits with status code 1 if there are no matches,
# so we manually set RESULT to 0 if nothing is found.
RESULT=$(grep -c "${COMPONENT}" .github/CODEOWNERS || true)

# there may be more than 1 component matching a label
# if so, try to narrow things down by appending the component
# or a forward slash to the label.
if [[ ${RESULT} != 1 ]]; then
    COMPONENT_TYPE=$(get_component_type "${COMPONENT}")
    OWNERS="$(get_codeowners "${COMPONENT}${COMPONENT_TYPE}")"
fi

if [[ -z "${OWNERS:-}" ]]; then
    OWNERS="$(get_codeowners "${COMPONENT}")"
fi

echo "${OWNERS}"
