#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

#
# verifies if the collector components are using the main core collector version
# as a dependency.
#

set -eu -o pipefail

CORE_REPO_RAW_BASE_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector"

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

get_collector_release_tag_candidates() {
   collector_mod_version="$1"
   # Strip pseudo-version suffix, if present.
   release_tag="${collector_mod_version%%-*}"
   echo "$release_tag"

   # For -0 pseudo versions, also try the previous patch tag.
   # Example: v0.146.2-0.20260219223409-abcdef012345 -> v0.146.1
   if [[ "$collector_mod_version" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)-0\.[0-9]{14}-[0-9a-f]+$ ]]; then
      patch="${BASH_REMATCH[3]}"
      if (( patch > 0 )); then
         echo "v${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((patch - 1))"
      fi
   fi
}

fetch_collector_versions_yaml() {
   collector_mod_version="$1"
   versions_file_path="$2"

   while IFS= read -r release_tag; do
      versions_url="${CORE_REPO_RAW_BASE_URL}/${release_tag}/versions.yaml"
      if curl --fail --silent --show-error --location --retry 3 --retry-delay 1 "$versions_url" --output "$versions_file_path"; then
         echo "$release_tag"
         return 0
      fi
   done < <(get_collector_release_tag_candidates "$collector_mod_version")

   echo "Error: failed to fetch opentelemetry-collector/versions.yaml for $collector_mod_version" >&2
   return 1
}

get_module_set_modules() {
   module_set="$1"
   versions_file_path="$2"

   awk -v module_set="$module_set" '
      /^  [^[:space:]][^:]*:/ {
         in_target_set = ($1 == module_set ":")
         in_modules = 0
      }
      in_target_set && $1 == "modules:" {
         in_modules = 1
         next
      }
      in_target_set && in_modules && /^      - / {
         sub(/^      - /, "")
         print
         next
      }
      in_target_set && in_modules && !/^      - / {
         in_modules = 0
      }
   ' "$versions_file_path"
}

# Compare the collector main core version against all the collector component
# modules to verify that they are using this version as its dependency
check_collector_versions_correct() {
   collector_module="$1"
   collector_mod_version="$2"
   echo "Checking $collector_module is used with $collector_mod_version"

   # Loop through all the module files, checking the collector version
   if [ "${GNU_SED_INSTALLED}" = false ]; then
      sed -i '' "s|$collector_module v[^ ]*|$collector_module $collector_mod_version|g" $mod_files
   else
      sed -i'' "s|$collector_module v[^ ]*|$collector_module $collector_mod_version|g" $mod_files
   fi
}

MAIN_MOD_FILE="./cmd/otelcontribcol/go.mod"

BETA_MODULE="go.opentelemetry.io/collector"
# Note space at end of string. This is so it filters for the exact string
# only and does not return string which contains this string as a substring.
BETA_MOD_VERSION=$(get_collector_version "$BETA_MODULE " "$MAIN_MOD_FILE")

collector_versions_yaml=$(mktemp)
trap 'rm -f "$collector_versions_yaml"' EXIT
selected_release_tag=$(fetch_collector_versions_yaml "$BETA_MOD_VERSION" "$collector_versions_yaml")
echo "Using module list from opentelemetry-collector $selected_release_tag"

check_collector_versions_correct "$BETA_MODULE" "$BETA_MOD_VERSION"
while IFS= read -r mod; do
   check_collector_versions_correct "$mod" "$BETA_MOD_VERSION"
done < <(get_module_set_modules "beta" "$collector_versions_yaml")

# Check stable modules using the stable module-set from collector versions.yaml.
STABLE_MODULE="go.opentelemetry.io/collector/pdata"
STABLE_MOD_VERSION=$(get_collector_version "$STABLE_MODULE" "$MAIN_MOD_FILE")
check_collector_versions_correct "$STABLE_MODULE" "$STABLE_MOD_VERSION"
while IFS= read -r mod; do
   check_collector_versions_correct "$mod" "$STABLE_MOD_VERSION"
done < <(get_module_set_modules "stable" "$collector_versions_yaml")

git diff --exit-code
