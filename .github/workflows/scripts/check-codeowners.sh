#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

#
# verifies:
# 1. That vendor components are assigned to owner(s)
# 2. That list of vendor components (in $CODEOWNERS) still exists
# in the project
#
set -eu -o pipefail

CODEOWNERS=".github/CODEOWNERS"
ALLOWLIST=".github/ALLOWLIST"

# Get component folders from the project and checks that they have
# an owner in $CODEOWNERS
check_code_owner_existence() {
  MODULES=$(find . -type f -name "go.mod" -exec dirname {} \; | sort | grep -E '^./' | cut -c 3-)
  MISSING_COMPONENTS=0
  ALLOW_LIST_COMPONENTS=0
  for module in ${MODULES}
  do
    # For a component path exact match, need to add '/ ' to end of module as
    # each line in the CODEOWNERS file is of the format:
    # <component_path_relative_from_project_root>/<min_1_space><owner_1><space><owner_2><space>..<owner_n>
    # This is because the path separator at end is dropped while searching for
    # modules and there is at least 1 space separating the path from the owners.
    if ! grep -q "^$module/ " "$CODEOWNERS"; then
      # If there is not an exact match to component path, there might be a parent folder
      # which has an owner and would therefore implicitly include the component
      # path as a sub folder e.g. 'internal/aws' is listed in $CODEOWNERS
      # which accounts for internal/aws/awsutil, internal/aws/k8s etc.
      PREFIX_MODULE_PATH=$(echo $module | cut -d/ -f 1-2)
      if ! grep -wq "^$PREFIX_MODULE_PATH/ " "$CODEOWNERS"; then
        # Check if it is a known component that is waiting on an owner
        if grep -wq "$module" "$ALLOWLIST"; then
          ((ALLOW_LIST_COMPONENTS=ALLOW_LIST_COMPONENTS+1))
          echo "pass: \"$module\" not included in CODEOWNERS but in the ALLOWLIST"
        else
          ((MISSING_COMPONENTS=MISSING_COMPONENTS+1))
          echo "FAIL: \"$module\" not included in CODEOWNERS"
        fi
      fi
    fi
  done
  if [ "$ALLOW_LIST_COMPONENTS" -gt 0 ]; then
    echo "---"
    echo "pass: there are $ALLOW_LIST_COMPONENTS components not included in CODEOWNERS but known in the ALLOWLIST"
  fi
  if [ "$MISSING_COMPONENTS" -gt 0 ]; then
    echo "---"
    echo "FAIL: there are $MISSING_COMPONENTS components not included in CODEOWNERS and not known in the ALLOWLIST"
    exit 1
  fi
}

# Checks that components specified in $CODEOWNERS still exist in the project
check_component_existence() {
  NOT_EXIST_COMPONENTS=0
  while IFS= read -r line
  do
    if [[ $line =~ ^[^#\*] ]]; then
      COMPONENT_PATH=$(echo "$line" | cut -d" " -f1)
      if [ ! -e "$COMPONENT_PATH" ]; then
        echo "\"$COMPONENT_PATH\" does not exist as specified in CODEOWNERS"
        ((NOT_EXIST_COMPONENTS=NOT_EXIST_COMPONENTS+1))
      fi
    fi
  done <"$CODEOWNERS"
  echo "there are $NOT_EXIST_COMPONENTS component(s) that do not exist as specified in CODEOWNERS"
  if [ "$NOT_EXIST_COMPONENTS" -gt 0 ]; then
    exit 1
  fi
}

check_entries_in_allowlist() {
  NOT_ORPHANED=0
  while IFS= read -r line
  do
    if [[ $line =~ ^[^#] ]]; then
      COMPONENT_PATH=$(echo "$line" | cut -d" " -f1)
      if grep -wq "^$COMPONENT_PATH/ " "$CODEOWNERS"; then
        echo "\"$COMPONENT_PATH\" has an entry in CODEOWNERS file"
        ((NOT_ORPHANED=NOT_ORPHANED+1))
      fi
    fi
  done <"$ALLOWLIST"
  echo "There are $NOT_ORPHANED component(s) that have owners but are present in ALLOWLIST file"
  if [ "$NOT_ORPHANED" -gt 0 ]; then
    exit 1
  fi
}

if [[ "$1" == "check_code_owner_existence" ]];  then
  check_code_owner_existence
elif [[ "$1" == "check_component_existence" ]]; then
  check_component_existence
elif [[ "$1" == "check_entries_in_allowlist" ]]; then
  check_entries_in_allowlist
fi

