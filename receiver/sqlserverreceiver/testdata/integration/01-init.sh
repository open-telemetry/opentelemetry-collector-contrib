#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Wait for SQL Server to start up
echo "Waiting for SQL Server to start..."
sleep 20

# Define credentials and parameters
SQLCMD="/opt/mssql-tools18/bin/sqlcmd -C -S localhost -U SA -P $MSSQL_SA_PASSWORD"

# Create the database
echo "Creating database 'mydb'..."
$SQLCMD -Q "IF DB_ID('mydb') IS NULL CREATE DATABASE mydb;"

# Create login and user, grant access
echo "Creating login, user, and granting access..."
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


# Create table and insert data
echo "Creating table and inserting data..."
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

echo "Creating otel collector user"
$SQLCMD -Q "
CREATE LOGIN otelcollectoruser WITH PASSWORD = 'otel-password123';
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT CONNECT ANY DATABASE to otelcollectoruser;
GRANT VIEW SERVER STATE to otelcollectoruser;
GRANT VIEW ANY DEFINITION to otelcollectoruser;

USE msdb;
CREATE USER otelcollectoruser FOR LOGIN otelcollectoruser;
GRANT SELECT to otelcollectoruser;
"

echo "Initialization complete."
