#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e
echo "Enabling pg_stat_statements"
psql -v ON_ERROR_STOP=1 --username "root" --dbname "postgres" <<-EOSQL
  CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
EOSQL

psql -v ON_ERROR_STOP=1 --username "root" --dbname "otel2" <<-EOSQL
  CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
  GRANT SELECT ON test2 TO otelu
EOSQL