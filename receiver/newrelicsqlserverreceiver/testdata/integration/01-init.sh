#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

TIMEOUT=60

SQLCMD="/opt/mssql-tools18/bin/sqlcmd -C -S localhost -U SA -P $MSSQL_SA_PASSWORD"

for ((i=0; i<TIMEOUT; i++)); do
    $SQLCMD -Q "SELECT 1" -b -l 1 >/dev/null 2>&1
    if [[ $? -eq 0 ]]; then
        break
    fi
    sleep 1
done

set -euo pipefail

$SQLCMD -Q "IF DB_ID('newrelicdb') IS NULL CREATE DATABASE newrelicdb;"

$SQLCMD -d newrelicdb -Q "
IF NOT EXISTS (SELECT name FROM sys.server_principals WHERE name = 'newrelicuser')
    BEGIN
        CREATE LOGIN newrelicuser WITH PASSWORD = 'NewRelicPass123!';
    END;
IF NOT EXISTS (SELECT name FROM sys.database_principals WHERE name = 'newrelicuser')
    BEGIN
        CREATE USER newrelicuser FOR LOGIN newrelicuser;
        ALTER ROLE db_owner ADD MEMBER newrelicuser;
    END;
"

$SQLCMD -d newrelicdb -Q "
IF OBJECT_ID('dbo.performance_test_table', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.performance_test_table (
        id INT PRIMARY KEY IDENTITY(1,1),
        name NVARCHAR(100),
        created_at DATETIME2 DEFAULT GETDATE()
    );
END;
INSERT INTO dbo.performance_test_table (name) VALUES 
    (N'Performance Test 1'), 
    (N'Performance Test 2'), 
    (N'Blocking Session Test'),
    (N'Query Monitoring Test');
"

# Create the otelcollectoruser for the receiver
$SQLCMD -Q "
CREATE LOGIN otelcollectoruser WITH PASSWORD = 'otel-password123';
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT VIEW SERVER PERFORMANCE STATE to otelcollectoruser;
GRANT VIEW SERVER STATE to otelcollectoruser;
GRANT VIEW DATABASE STATE to otelcollectoruser;
"

echo "Initialization complete."
