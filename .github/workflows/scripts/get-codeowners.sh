#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# This script checks the GitHub CODEOWNERS file for any code owners
# of contrib components and returns a string of the code owners if it
# finds them.

set -euo pipefail

if [[ -z "${COMPONENT:-}" ]]; then
    echo "COMPONENT has not been set, please ensure it is set."
    exit 1
fi

# grep exits with status code 1 if there are no matches,
# so we manually set RESULT to 0 if nothing is found.
RESULT=$(grep -c "${COMPONENT}" .github/CODEOWNERS || true)

# there may be more than 1 component matching a label
# if so, try to narrow things down by appending the component
# type to the label
if [[ ${RESULT} != 1 ]]; then
    COMPONENT_TYPE=$(echo "${COMPONENT}" | cut -f 1 -d '/')
    COMPONENT="${COMPONENT}${COMPONENT_TYPE}"
fi

OWNERS=$( (grep -m 1 "${COMPONENT}" .github/CODEOWNERS || true) | sed 's/   */ /g' | cut -f3- -d ' ' )

echo "${OWNERS}"

