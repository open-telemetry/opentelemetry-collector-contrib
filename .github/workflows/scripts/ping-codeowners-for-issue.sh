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
# This file takes a comma-delimited list of labels and uses them to ping
# code owners of labels corresponding to a Collector contrib component.

set -euo pipefail

if [ -z "${LABELS}" ] || [ -z "${ISSUE}" ]; then
    exit 0
fi

OWNERS=''

IFS=',' read -ra LIST <<< "$LABELS"
for LABEL in "${LIST[@]}"; do
    if ! [[ ${LABEL} =~ ^cmd/ || ${LABEL} =~ ^confmap/ || ${LABEL} =~ ^exporter/ || ${LABEL} =~ ^extension/ || ${LABEL} =~ ^internal/ || ${LABEL} =~ ^pkg/ || ${LABEL} =~ ^processor/ || ${LABEL} =~ ^receiver/ ]]; then
        continue
    fi

    COMPONENT=${LABEL}
    result=`grep -c ${LABEL} .github/CODEOWNERS`

    # there may be more than 1 component matching a label
    # if so, try to narrow things down by appending the component
    # type to the label
    if [[ $result != 1 ]]; then
        COMPONENT_TYPE=`echo ${COMPONENT} | cut -f 1 -d '/'`
        COMPONENT="${COMPONENT}${COMPONENT_TYPE}"
    fi

    OWNERS+="- ${COMPONENT}: `grep -m 1 ${COMPONENT} .github/CODEOWNERS | sed 's/   */ /g' | cut -f3- -d ' '`\n"
done

if [ -z "${OWNERS}" ]; then
    exit 0
fi

# The GitHub CLI only offers multiline strings by specifying stdin as file input
printf "Pinging code owners:\n${OWNERS}\nSee [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments) if you do not have permissions to add labels yourself." \
  | gh issue comment ${ISSUE} -F -
