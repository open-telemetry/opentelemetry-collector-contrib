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

COMPONENT_TYPE=$(get_component_type "${COMPONENT}")
OWNERS="$(get_codeowners "${COMPONENT}${COMPONENT_TYPE}")"

if [[ -z "${OWNERS:-}" ]]; then
    OWNERS="$(get_codeowners "${COMPONENT}")"
fi

echo "${OWNERS}"
