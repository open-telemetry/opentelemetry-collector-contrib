#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

#
# verifies if any changed file is related to windows architecture.
# this script is used to determine if e2e tests should be run on windows.
#
# It's intended to be used in a GitHub Actions workflow:
# bash .github/workflows/scripts/is_changed_file_windows.sh ${{ github.event.pull_request.base.sha }} ${{ github.sha }}


# Get changed files
base_sha=$1
head_sha=$2
changed_files=$(git diff --name-only $base_sha $head_sha)

# Find windows related files
windows_files=$(find * -regex ".*windows.*.go")


# Loop over changed files and check if they exist in windows_files
found_windows_file=false
for file in $changed_files; do
  for windows_file in $windows_files; do
    if [[ $file == "$windows_file" ]]; then
      found_windows_file=true
      break
    fi
  done
done

echo "$found_windows_file"