#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

#
# verifies if the collector components are using the main core collector version
# as a dependency.
#

source ./internal/buildscripts/modules

set -eu -o pipefail

# Return the collector main core version
get_collector_version() {
   collector_module="$1"
   main_mod_file="$2"

   if grep -q "$collector_module" "$main_mod_file"; then
      grep "$collector_module" "$main_mod_file" | (read mod version rest;
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
         mod_line=$(grep -m1 "$collector_module" "$mod_file")
         version=$(echo "$mod_line" | cut -d" " -f2)

         # To account for a module on its own 'require' line,
         # the version field is shifted right by 1. Match
         # with or without a trailing space at the end to account
         # for the space at the end of some collector modules.
         if [ "$version" == "$collector_module" ] || [ "$version " == "$collector_module" ]; then
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

MAIN_MOD_FILE="./go.mod"

# Note space at end of string. This is so it filters for the exact string
# only and does not return string which contains this string as a substring.
BETA_MODULE="go.opentelemetry.io/collector "
BETA_MOD_VERSION=$(get_collector_version "$BETA_MODULE" "$MAIN_MOD_FILE")
check_collector_versions_correct "$BETA_MODULE" "$BETA_MOD_VERSION"
for mod in ${beta_modules[@]}; do
   check_collector_versions_correct "$mod" "$BETA_MOD_VERSION"
done

# Check RC modules
RC_MODULE="go.opentelemetry.io/collector/pdata "
RC_MOD_VERSION=$(get_collector_version "$RC_MODULE" "$MAIN_MOD_FILE")
check_collector_versions_correct "$RC_MODULE" "$RC_MOD_VERSION"
for mod in ${rc_modules[@]}; do
   check_collector_versions_correct "$mod" "$RC_MOD_VERSION"
done

# Check stable modules, none currently exist, uncomment when pdata is 1.0.0
# STABLE_MODULE="go.opentelemetry.io/collector/pdata "
# STABLE_MOD_VERSION=$(get_collector_version "$STABLE_MODULE" "$MAIN_MOD_FILE")
# check_collector_versions_correct "$STABLE_MODULE" "$STABLE_MOD_VERSION"
# for mod in ${stable_modules[@]}; do
#    check_collector_versions_correct "$mod" "$STABLE_MOD_VERSION"
# done