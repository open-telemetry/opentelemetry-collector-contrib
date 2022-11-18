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

declare -A COLORS

COLORS["cmd"]="#483C32"
COLORS["confmap"]="#6666FF"
COLORS["examples"]="#1A8CFF"
COLORS["exporter"]="#50C878"
COLORS["extension"]="#FF794D"
COLORS["internal"]="#B30059"
COLORS["pkg"]="#F9DE22"
COLORS["processor"]="#800080"
COLORS["receiver"]="#E91B7B"
COLORS["testbed"]="#336600"

FALLBACK_COLOR="#999966"

# Match only components with a type and name, i.e. no top-level
# directories.
COMPONENTS=$(grep -oE '^[a-z]+/[a-z/]+ ' < .github/CODEOWNERS)

for COMPONENT in ${COMPONENTS}; do
		COMPONENT=$(echo "${COMPONENT}" | sed -E 's%/$%%')
		TYPE=$(echo "${COMPONENT}" | cut -f1 -d '/' )
		NAME=$(echo "${COMPONENT}" | cut -f2- -d '/' | sed -E "s%${TYPE}\$%%")

		gh label create "${TYPE}/${NAME}" -c "${COLORS["${TYPE}"]:-${FALLBACK_COLOR}}" --force
done

