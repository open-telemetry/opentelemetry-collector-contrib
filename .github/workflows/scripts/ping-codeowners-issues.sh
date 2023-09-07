#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
#

set -euo pipefail

if [[ -z "${COMPONENT:-}" || -z "${ISSUE:-}" || -z "${SENDER:-}" ]]; then
    echo "At least one of COMPONENT, ISSUE, or SENDER has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")

OWNERS=$(COMPONENT="${COMPONENT}" bash "${CUR_DIRECTORY}/get-codeowners.sh")

if [[ -z "${OWNERS}" ]]; then
    exit 0
fi

if [[ "${OWNERS}" =~ "${SENDER}" ]]; then
    echo "Label applied by code owner ${SENDER}"
    exit 0
fi

gh issue comment "${ISSUE}" --body "Pinging code owners for ${COMPONENT}: ${OWNERS}. See [Adding Labels via Comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-labels-via-comments) if you do not have permissions to add labels yourself."
