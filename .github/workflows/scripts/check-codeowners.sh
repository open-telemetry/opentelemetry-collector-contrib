#!/usr/bin/env bash

#   Copyright The OpenTelemetry Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

#
# verifies:
# 1. That vendor components are assigned to owner(s)
# 2. That list of vendor components (in $CODEOWNERS) still exists
# in the project
#
set -eu -o pipefail

CODEOWNERS=".github/CODEOWNERS"

# Get component folders from the project and checks that they have 
# an owner in $CODEOWNERS 
check_code_owner_existence() {
  MODULES=$(find . -type f -name "go.mod" -exec dirname {} \; | sort | grep -E '^./' | cut -c 3-)
  MISSING_COMPONENTS=0
  for module in ${MODULES}
  do
    if ! grep -q "^$module" "$CODEOWNERS"; then
      # Account for parent folders which implicitly include 
      # sub folders e.g. 'internal/aws' is listed in $CODEOWNERS
      # which accounts for internal/aws/awsutil, internal/aws/k8s etc.
      PREFIX_MODULE_PATH=$(echo $module | cut -d/ -f 1-2)
      if ! grep -wq "^$PREFIX_MODULE_PATH/ " "$CODEOWNERS"; then
        ((MISSING_COMPONENTS=MISSING_COMPONENTS+1))
        echo "\"$module\" not included in CODEOWNERS"
      fi
    fi
  done
  echo "there are $MISSING_COMPONENTS components not included in CODEOWNERS"
  if [ "$MISSING_COMPONENTS" -gt 0 ]; then
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
      if [ ! -d "$COMPONENT_PATH" ]; then
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

if [[ "$1" == "check_code_owner_existence" ]];  then
  check_code_owner_existence
elif [[ "$1" == "check_component_existence" ]]; then
  check_component_existence
fi