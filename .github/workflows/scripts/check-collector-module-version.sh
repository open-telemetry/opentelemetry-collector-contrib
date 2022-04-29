#!/usr/bin/env bash

#   Copyright The OpenTelemetry Authors

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
# verifies if the collector components are using the main core collector version
# as a dependency.
#
set -eu -o pipefail

# Return the collector main core version
get_collector_version() {
   collector_module="$1"
   main_mod_file="$2"

   if grep -q "$collector_module" "$main_mod_file"; then
      grep "$collector_module" "$main_mod_file" | (read mod version;
         echo $version)
   else
      echo "Error: failed to retrieve the \"$collector_module\" version from \"$main_mod_file\"."
      exit 1
   fi
}

# Compare the collector main core version against all the collector component
# modules to verify that they are using this version as its dependency
check_collector_versions_correct() {
   collector_module="$1"
   collector_mod_version="$2"
   incorrect_version=0
   mod_files=$(find . -type f -name "go.mod")

   # Loop through all the module files, checking the collector version
   for mod_file in $mod_files; do
      if grep -q "$collector_module" "$mod_file"; then
         mod_line=$(grep "$collector_module" "$mod_file")
         version=$(echo "$mod_line" | cut -d" " -f2)
         
         # To account for a module on its own 'require' line,
         # the version field is shifted right by 1
         if [ "$version" == "$collector_module" ]; then
            version=$(echo "$mod_line" | cut -d" " -f3)
         fi

         if [ "$version" != "$collector_mod_version" ]; then
            incorrect_version=$((incorrect_version+1))
            echo "Incorrect version \"$version\" of \"$collector_module\" is included in \"$mod_file\". It should be version \"$collector_mod_version\"."
         fi
      fi
   done

   echo "There are $incorrect_version incorrect \"$collector_module\" version(s) in the module files."
   if [ "$incorrect_version" -gt 0 ]; then
      exit 1
   fi
}

# Note space at end of string. This is so it filters for the exact string
# only and does not return string which contains this string as a substring.
COLLECTOR_MODULE="go.opentelemetry.io/collector "

COLLECTOR_MODEL_PDATA="go.opentelemetry.io/collector/pdata"
COLLECTOR_MODEL_SEMCONV="go.opentelemetry.io/collector/semconv"
MAIN_MOD_FILE="./go.mod"
COLLECTOR_MOD_VERSION=$(get_collector_version "$COLLECTOR_MODULE" "$MAIN_MOD_FILE")

# Check the collector module version in each of the module files
check_collector_versions_correct "$COLLECTOR_MODULE" "$COLLECTOR_MOD_VERSION"

# Check the collector model module version in each of the module files
check_collector_versions_correct "$COLLECTOR_MODEL_PDATA" "$COLLECTOR_MOD_VERSION"
check_collector_versions_correct "$COLLECTOR_MODEL_SEMCONV" "$COLLECTOR_MOD_VERSION"
