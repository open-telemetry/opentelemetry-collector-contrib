#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

USER="otel"
ROOT_PASS="otel"

# NOTE: -pPASSWORD is missing a space on purpose
mysql -u root -p"${ROOT_PASS}" -e "GRANT PROCESS ON *.* TO '${USER}'@'%'" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "GRANT SELECT ON information_schema.innodb_metrics TO '${USER}'@'%'" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "GRANT SELECT ON performance_schema.table_io_waits_summary_by_table TO '${USER}'@'%'" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "GRANT SELECT ON performance_schema.table_io_waits_summary_by_index_usage TO '${USER}'@'%'" > /dev/null
mysql -u root -p"${ROOT_PASS}" -e "FLUSH PRIVILEGES" > /dev/null
