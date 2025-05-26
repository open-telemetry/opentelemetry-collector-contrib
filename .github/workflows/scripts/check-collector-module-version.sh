#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

#
# verifies if the collector components are using the main core collector version
# as a dependency.
#

source ./internal/buildscripts/modules

set -eu -o pipefail

mod_files=$(find . -type f -name "go.mod")

# Check if GNU sed is installed
GNU_SED_INSTALLED=false
if sed --version 2>/dev/null | grep -q "GNU sed"; then
   GNU_SED_INSTALLED=true
fi

# Return the collector main core version
get_collector_version() {
   collector_module="$1"
   main_mod_file="$2"

   if grep -q "$collector_module" "$main_mod_file"; then
      grep "$collector_module" "$main_mod_file" | (read -r mod version rest;
         echo "$version")
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
   echo "Checking $collector_module is used with $collector_mod_version"

   # Loop through all the module files, checking the collector version
   if [ "${GNU_SED_INSTALLED}" = false ]; then
      sed -i '' "s|$collector_module [^ ]*|$collector_module $collector_mod_version|g" $mod_files
   else
      sed -i'' "s|$collector_module [^ ]*|$collector_module $collector_mod_version|g" $mod_files
   fi
}

MAIN_MOD_FILE="./cmd/otelcontribcol/go.mod"


BETA_MODULE="go.opentelemetry.io/collector"
# Note space at end of string. This is so it filters for the exact string
# only and does not return string which contains this string as a substring.
BETA_MOD_VERSION=$(get_collector_version "$BETA_MODULE " "$MAIN_MOD_FILE")
check_collector_versions_correct "$BETA_MODULE" "$BETA_MOD_VERSION"
for mod in "${beta_modules[@]}"; do
   check_collector_versions_correct "$mod" "$BETA_MOD_VERSION"
done

# Check stable modules, none currently exist, uncomment when pdata is 1.0.0
STABLE_MODULE="go.opentelemetry.io/collector/pdata"
STABLE_MOD_VERSION=$(get_collector_version "$STABLE_MODULE" "$MAIN_MOD_FILE")
check_collector_versions_correct "$STABLE_MODULE" "$STABLE_MOD_VERSION"
for mod in "${stable_modules[@]}"; do
   check_collector_versions_correct "$mod" "$STABLE_MOD_VERSION"
done

git diff --exit-code
