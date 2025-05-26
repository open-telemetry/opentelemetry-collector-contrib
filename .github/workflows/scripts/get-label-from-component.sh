#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# This script is used to get the component label from a given COMPONENT.
# The key here is to match a single component based on the input component,
# and match it exactly.
#

set -euo pipefail

get_label() {
  MATCHING_LABELS=$(awk -v path="${COMPONENT}" 'index($1, path) > 0 || index($2, path) > 0 {print $2}' .github/component_labels.txt)

  if [ -z "${MATCHING_LABELS}" ]; then
    echo ""
    return
  fi

  LABEL_NAME=""
  for POTENTIAL_LABEL in ${MATCHING_LABELS}; do
    if [ "$POTENTIAL_LABEL" = "$COMPONENT" ]; then
      LABEL_NAME=$POTENTIAL_LABEL
      break
    fi
  done

  if [ -z "${LABEL_NAME}" ]; then
    LABEL_NAME="${MATCHING_LABELS[0]}"
  fi

  echo $LABEL_NAME
}

if [[ -z "${COMPONENT:-}" ]]; then
    echo "COMPONENT has not been set, please ensure it is set."
    exit 1
fi

echo "$(get_label)"
