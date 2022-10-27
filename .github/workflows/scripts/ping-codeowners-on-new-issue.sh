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
#

set -euo pipefail

if [[ -z "${ISSUE:-}" || -z "${TITLE:-}" || -z "${BODY:-}" || -z "${OPENER:-}" ]]; then
  echo "Missing one of ISSUE, TITLE, BODY, or OPENER, please ensure all are set."
  exit 0
fi

COMPONENT_REGEX='(cmd|pkg|extension|receiver|processor|exporter)'
CUR_DIRECTORY=`dirname $0`
LABELS=""
PING_LINES=""
declare -A ADDED_LABELS

TITLE_COMPONENT=`echo "${TITLE}" | grep -oE "\[${COMPONENT_REGEX}/.+\]" | sed -E "s%^\[(${COMPONENT_REGEX}/.+)\]%\1%" || true`

COMPONENTS_SECTION_START=`(echo "${BODY}" | grep -n '### Component(s)' | awk '{ print $1 }' | grep -oE '[0-9]+') || echo '-1'`
BODY_COMPONENTS=""

if [[ "${COMPONENTS_SECTION_START}" != '-1' ]]; then
  BODY_COMPONENTS=`echo "${BODY}" | sed -n $((${COMPONENTS_SECTION_START}+2))p`
fi

if [[ -n "${TITLE_COMPONENT}" && ! ("${TITLE_COMPONENT}" =~ " ") ]]; then
  # Transforms e.g. exporter/otlpexporter into exporter/otlp
  TITLE_COMPONENT=`echo "${TITLE_COMPONENT}" | sed -E "s%${COMPONENT_REGEX}/(.+)${COMPONENT_REGEX}%\1/\2%"`
  CODEOWNERS=`COMPONENT="${TITLE_COMPONENT}" "${CUR_DIRECTORY}/get-codeowners.sh" || true`
  
  if [[ -n "${CODEOWNERS}" && ! ("${CODEOWNERS}" =~ "${OPENER}") ]]; then
    ADDED_LABELS["${TITLE_COMPONENT}"]=1
    LABELS+="${TITLE_COMPONENT}"
    PING_LINES+="- ${TITLE_COMPONENT}: ${CODEOWNERS}\n"
  fi
fi

for COMPONENT in ${BODY_COMPONENTS}; do
  COMPONENT=`echo "${COMPONENT}" | sed 's/,//'`
  
  CODEOWNERS=`COMPONENT="${COMPONENT}" "${CUR_DIRECTORY}/get-codeowners.sh" || true`
  
  if [[ -n "${CODEOWNERS}" && ! ("${CODEOWNERS}" =~ "${OPENER}") ]]; then
    if [[ -v ADDED_LABELS["${COMPONENT}"] ]]; then
      continue
    fi
    if [[ -n "${LABELS}" ]]; then
      LABELS+=","
    fi
    LABELS+="${COMPONENT}"
    PING_LINES+="- ${COMPONENT}: ${CODEOWNERS}\n"
  fi
done

if [[ -n "${LABELS}" ]]; then
  echo "${LABELS}"
  echo -e "${PING_LINES}"
  gh issue edit "${ISSUE}" --add-label "${LABELS}"
  # The GitHub CLI only offers multiline strings through file input.
  # The CLI may also fail if a tag doesn't exist, so stop this from failing the job.
   printf "Pinging code owners:\n${PING_LINES}\nSee [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments) if you do not have permissions to add labels yourself." \
   | gh issue comment ${ISSUE} -F - || true
fi
