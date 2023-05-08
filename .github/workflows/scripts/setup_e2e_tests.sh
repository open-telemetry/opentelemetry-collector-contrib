#!/bin/bash

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

TESTS="$(make -s -C testbed list-tests | xargs echo|sed 's/ /|/g')"
TESTS=(${TESTS//|/ })
MATRIX="{\"include\":["
curr=""
for i in "${!TESTS[@]}"; do
if (( i > 0 && i % 2 == 0 )); then
    curr+="|${TESTS[$i]}"
else
    if [ -n "$curr" ] && (( i>1 )); then
    MATRIX+=",{\"test\":\"$curr\"}"
    elif [ -n "$curr" ]; then
    MATRIX+="{\"test\":\"$curr\"}"
    fi
    curr="${TESTS[$i]}"
fi
done
MATRIX+=",{\"test\":\"$curr\"}]}"
echo "loadtest_matrix=$MATRIX" >> $GITHUB_OUTPUT
