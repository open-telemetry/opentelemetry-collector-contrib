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

generate-labels() {
	TYPE=$1
	COLOR=$2

	echo "Generating labels for ${TYPE}"
	COMPONENTS=$(find "./${TYPE}" -maxdepth 1 -mindepth 1 -type d -exec basename \{\} \;)
	for COMPONENT in ${COMPONENTS}; do
		NAME=${COMPONENT//"${TYPE}"}
		gh label create "${TYPE}/${NAME}" -c "${COLOR}" --force
	done
}

generate-labels "cmd" "#483C32"
generate-labels "pkg" "#F9DE22"
generate-labels "extension" "#FF794D"
generate-labels "receiver" "#E91B7B"
generate-labels "processor" "#800080"
generate-labels "exporter" "#50C878"

