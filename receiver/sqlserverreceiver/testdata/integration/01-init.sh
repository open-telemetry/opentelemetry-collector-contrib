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

$SQLCMD -d mydb -Q "
IF OBJECT_ID('dbo.test_indexed_table', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.test_indexed_table (
        id       INT           NOT NULL PRIMARY KEY IDENTITY(1,1),
        name     NVARCHAR(100) NOT NULL,
        category NVARCHAR(50)  NOT NULL
    );
    CREATE NONCLUSTERED INDEX IX_test_indexed_table_name
        ON dbo.test_indexed_table (name) INCLUDE (category);
END;

-- Insert enough rows so sys.dm_db_index_physical_stats returns data under SAMPLED mode
DECLARE @i INT = 0;
WHILE @i < 500
BEGIN
    INSERT INTO dbo.test_indexed_table (name, category)
    VALUES (CONCAT(N'item-', @i), CONCAT(N'cat-', @i % 10));
    SET @i = @i + 1;
END;
"

$SQLCMD -Q "
CREATE LOGIN otelcollectoruser WITH PASSWORD = 'otel-password123';
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT VIEW SERVER PERFORMANCE STATE to otelcollectoruser;
"

# The index physical stats query loops over user databases and runs dynamic SQL inside each
# one under the login's own security context, joining sys.dm_db_index_physical_stats against
# the per-database sys.indexes/objects/schemas catalog views. The login therefore needs, in
# each target database: a database user (so it can enter the database), VIEW DEFINITION (so
# the dbo-owned tables/indexes are visible in the catalog views), and VIEW DATABASE
# PERFORMANCE STATE (so the DMF returns rows). Without all three the per-database query yields
# no index metrics.
$SQLCMD -d mydb -Q "
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT VIEW DEFINITION TO otelcollectoruser;
GRANT VIEW DATABASE PERFORMANCE STATE TO otelcollectoruser;
"

echo "Initialization complete."
