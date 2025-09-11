#!/bin/bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
#


# Start the script to create the DB and user
/usr/src/app/configure-db.sh &

# Start SQL Server
/opt/mssql/bin/sqlservr