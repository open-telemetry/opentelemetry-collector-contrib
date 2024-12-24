#!/bin/bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
#


# Start SQL Server in the background
/opt/mssql/bin/sqlservr &

# Wait for SQL Server to start
sleep 30s

# Run the setup script to create the DB and the schema in the DB
/usr/src/app/configure-db.sh

# Call the original entrypoint script
wait