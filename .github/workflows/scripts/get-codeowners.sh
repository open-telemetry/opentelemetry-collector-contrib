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

