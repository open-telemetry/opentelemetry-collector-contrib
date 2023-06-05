#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

USER="otel"
ROOT_PASS="otel"

# # NOTE: -pPASSWORD is missing a space on purpose
mysql -u root -p"${ROOT_PASS}" -e "GRANT PROCESS ON *.* TO ${USER}" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "GRANT SELECT ON performance_schema.* TO ${USER}" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "FLUSH PRIVILEGES" > /dev/null
