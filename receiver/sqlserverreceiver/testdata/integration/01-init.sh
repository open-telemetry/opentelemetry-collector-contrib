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

$SQLCMD -Q "IF DB_ID('mydb') IS NULL CREATE DATABASE mydb;"

$SQLCMD -d mydb -Q "
IF NOT EXISTS (SELECT name FROM sys.server_principals WHERE name = 'myuser')
    BEGIN
        CREATE LOGIN myuser WITH PASSWORD = 'UserStrongPass1';
    END;
IF NOT EXISTS (SELECT name FROM sys.database_principals WHERE name = 'myuser')
    BEGIN
        CREATE USER myuser FOR LOGIN myuser;
        ALTER ROLE db_owner ADD MEMBER myuser;
    END;
"

$SQLCMD -d mydb -Q "
IF OBJECT_ID('dbo.test_table', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.test_table (
        id INT PRIMARY KEY IDENTITY(1,1),
        name NVARCHAR(100)
    );
END;
INSERT INTO dbo.test_table (name) VALUES (N'Hello World'), (N'Test Entry');
"

$SQLCMD -Q "
CREATE LOGIN otelcollectoruser WITH PASSWORD = 'otel-password123';
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT VIEW SERVER PERFORMANCE STATE to otelcollectoruser;
"

echo "Initialization complete."
