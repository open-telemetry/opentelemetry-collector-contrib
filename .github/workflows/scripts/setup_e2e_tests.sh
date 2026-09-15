#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

TESTS="$(make -s -C testbed list-tests | xargs echo|sed 's/ /|/g')"
IFS='|' read -r -a TEST_ARRAY <<< "$TESTS"

MATRIX="{\"include\":["
curr=""
for i in "${!TEST_ARRAY[@]}"; do
  if (( i > 0 && i % 2 == 0 )); then
    curr+="|${TEST_ARRAY[$i]}"
  else
    if [ -n "$curr" ] && (( i>1 )); then
      MATRIX+=",{\"test\":\"$curr\",\"runner\":\"oracle-bare-metal-64cpu-1024gb-x86-64-ubuntu-24\",\"type\":\"linux-amd64\"},{\"test\":\"$curr\",\"runner\":\"cncf-ubuntu-32-128-arm\",\"type\":\"linux-arm64\"}"
    elif [ -n "$curr" ]; then
      MATRIX+="{\"test\":\"$curr\",\"runner\":\"oracle-bare-metal-64cpu-1024gb-x86-64-ubuntu-24\",\"type\":\"linux-amd64\"},{\"test\":\"$curr\",\"runner\":\"cncf-ubuntu-32-128-arm\",\"type\":\"linux-arm64\"}"
    fi
    curr="${TEST_ARRAY[$i]}"
  fi
done
MATRIX+=",{\"test\":\"$curr\",\"runner\":\"oracle-bare-metal-64cpu-1024gb-x86-64-ubuntu-24\",\"type\":\"linux-amd64\"},{\"test\":\"$curr\",\"runner\":\"cncf-ubuntu-32-128-arm\",\"type\":\"linux-arm64\"}]}"
echo "loadtest_matrix=$MATRIX" >> "$GITHUB_OUTPUT"
