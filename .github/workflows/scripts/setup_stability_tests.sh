#!/bin/bash -ex

# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

TESTS="$(make -s -C testbed list-stability-tests | xargs echo|sed 's/ /|/g')"

TESTS=(${TESTS//|/ })
MATRIX="{\"include\":["
curr=""
for i in "${!TESTS[@]}"; do
    curr="${TESTS[$i]}"
    MATRIX+="{\"test\":\"$curr\"},"
done
MATRIX+="]}"
echo "stabilitytest_matrix=$MATRIX" >> $GITHUB_OUTPUT
