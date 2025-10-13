// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL queries for collecting SQL Server database principals metrics.
// This file implements comprehensive collection of database security principals information
// including users, roles, and related security metadata.
//
// Database Principals Overview:
//
// Database principals are security entities within a SQL Server database that can be
// granted permissions to access database objects. The sys.database_principals catalog
// view provides comprehensive information about all database-level security principals.
//
// Query Strategy:
// 1. Base Query: Retrieve all non-fixed role principals with essential metadata
// 2. Summary Queries: Aggregate principal counts by type for monitoring
// 3. Activity Queries: Track principal lifecycle and identify potential issues
// 4. Engine-Specific Variants: Handle differences between SQL Server editions
//
// Excluded Principals:
// - Fixed Database Roles (is_fixed_role = 1): Built-in roles like db_owner, db_datareader
// - Extended Properties Users (type = 'X'): Internal SQL Server entities
// - System Principals: Focus on user-created and application-specific principals
//
// Principal Types Covered:
// - S: SQL_USER (SQL Server authenticated users)
// - U: WINDOWS_USER (Windows/AD authenticated users)
// - R: DATABASE_ROLE (User-defined database roles)
// - A: APPLICATION_ROLE (Application-specific roles)
// - C: CERTIFICATE_MAPPED_USER (Certificate-based authentication)
// - K: ASYMMETRIC_KEY_MAPPED_USER (Asymmetric key-based authentication)
//
// Security Considerations:
// - Queries do not expose passwords or sensitive authentication data
// - Focus on metadata useful for auditing and monitoring
// - Support compliance requirements for access control visibility
//
// Engine Support:
// - Standard SQL Server: Full access to all principal types and metadata
// - Azure SQL Database: Database-scoped principals only
// - Azure SQL Managed Instance: Full functionality with enterprise features
//
// Query Source Inspiration:
// Based on the provided user query with enhancements for comprehensive monitoring:
// SELECT name AS principal_name, type_desc, create_date
// FROM sys.database_principals
// WHERE is_fixed_role = 0 AND type <> 'X'
// ORDER BY type_desc, principal_name
package queries

// DatabasePrincipalsQuery returns the SQL query for database principals information
// This query retrieves all non-fixed role database principals with their metadata
// Excludes system fixed roles and extended properties users for security auditing
const DatabasePrincipalsQuery = `SELECT 
    p.name AS principal_name,
    p.type_desc,
    p.create_date,
    DB_NAME() AS database_name
FROM sys.database_principals p
WHERE p.is_fixed_role = 0 
    AND p.type <> 'X'  -- Exclude extended properties users
    AND p.name NOT IN ('guest', 'INFORMATION_SCHEMA', 'sys')  -- Exclude additional system principals
ORDER BY p.type_desc, p.name`

// DatabasePrincipalsQueryAzureSQL returns the Azure SQL Database specific principals query
// Azure SQL Database has the same sys.database_principals view structure
const DatabasePrincipalsQueryAzureSQL = DatabasePrincipalsQuery

// DatabasePrincipalsQueryAzureMI returns the Azure SQL Managed Instance specific principals query
// Azure SQL Managed Instance supports the same functionality as standard SQL Server
const DatabasePrincipalsQueryAzureMI = DatabasePrincipalsQuery

// DatabasePrincipalsSummaryQuery returns aggregated statistics about database principals
// This query provides counts by principal type for monitoring and alerting
const DatabasePrincipalsSummaryQuery = `SELECT 
    DB_NAME() AS database_name,
    COUNT(*) AS total_principals,
    SUM(CASE WHEN type IN ('S', 'U', 'C', 'K') THEN 1 ELSE 0 END) AS user_count,
    SUM(CASE WHEN type IN ('R', 'A') THEN 1 ELSE 0 END) AS role_count,
    SUM(CASE WHEN type = 'S' THEN 1 ELSE 0 END) AS sql_user_count,
    SUM(CASE WHEN type = 'U' THEN 1 ELSE 0 END) AS windows_user_count,
    SUM(CASE WHEN type = 'A' THEN 1 ELSE 0 END) AS app_role_count
FROM sys.database_principals p
WHERE p.is_fixed_role = 0 
    AND p.type <> 'X'
    AND p.name NOT IN ('guest', 'INFORMATION_SCHEMA', 'sys')`

// DatabasePrincipalsSummaryQueryAzureSQL returns the Azure SQL Database specific summary query
const DatabasePrincipalsSummaryQueryAzureSQL = DatabasePrincipalsSummaryQuery

// DatabasePrincipalsSummaryQueryAzureMI returns the Azure SQL Managed Instance specific summary query
const DatabasePrincipalsSummaryQueryAzureMI = DatabasePrincipalsSummaryQuery

// DatabasePrincipalActivityQuery returns information about principal activity and lifecycle
// This query helps identify recently created principals and potential orphaned users
const DatabasePrincipalActivityQuery = `SELECT 
    DB_NAME() AS database_name,
    SUM(CASE WHEN create_date >= DATEADD(day, -30, GETDATE()) THEN 1 ELSE 0 END) AS recent_principals,
    SUM(CASE WHEN create_date < DATEADD(year, -1, GETDATE()) THEN 1 ELSE 0 END) AS old_principals,
    -- Orphaned users: SQL users without corresponding server logins
    (SELECT COUNT(*) 
     FROM sys.database_principals dp 
     WHERE dp.type = 'S' 
       AND dp.is_fixed_role = 0
       AND dp.name <> 'dbo'
       AND dp.principal_id > 4  -- Exclude system principals
       AND NOT EXISTS (
           SELECT 1 FROM sys.server_principals sp 
           WHERE sp.name = dp.name AND sp.type = 'S'
       )
    ) AS orphaned_users
FROM sys.database_principals p
WHERE p.is_fixed_role = 0 
    AND p.type <> 'X'
    AND p.name NOT IN ('guest', 'INFORMATION_SCHEMA', 'sys')`

// DatabasePrincipalActivityQueryAzureSQL returns the Azure SQL Database specific activity query
// Azure SQL Database may have different orphaned user detection logic
const DatabasePrincipalActivityQueryAzureSQL = `SELECT 
    DB_NAME() AS database_name,
    SUM(CASE WHEN create_date >= DATEADD(day, -30, GETDATE()) THEN 1 ELSE 0 END) AS recent_principals,
    SUM(CASE WHEN create_date < DATEADD(year, -1, GETDATE()) THEN 1 ELSE 0 END) AS old_principals,
    -- Simplified orphaned user detection for Azure SQL Database
    SUM(CASE WHEN type = 'S' AND name <> 'dbo' AND principal_id > 4 THEN 1 ELSE 0 END) AS orphaned_users
FROM sys.database_principals p
WHERE p.is_fixed_role = 0 
    AND p.type <> 'X'
    AND p.name NOT IN ('guest', 'INFORMATION_SCHEMA', 'sys')`

// DatabasePrincipalActivityQueryAzureMI returns the Azure SQL Managed Instance specific activity query
// Azure SQL Managed Instance supports the same orphaned user detection as standard SQL Server
const DatabasePrincipalActivityQueryAzureMI = DatabasePrincipalActivityQuery

// DatabasePrincipalDetailQuery returns detailed information about a specific principal
// This query can be used for troubleshooting or detailed principal analysis
const DatabasePrincipalDetailQuery = `SELECT 
    p.name AS principal_name,
    p.type_desc,
    p.create_date,
    p.modify_date,
    p.principal_id,
    p.type,
    p.is_fixed_role,
    p.default_schema_name,
    CASE 
        WHEN p.type = 'S' THEN 'SQL User'
        WHEN p.type = 'U' THEN 'Windows User'
        WHEN p.type = 'R' THEN 'Database Role'
        WHEN p.type = 'A' THEN 'Application Role'
        WHEN p.type = 'C' THEN 'Certificate Mapped User'
        WHEN p.type = 'K' THEN 'Asymmetric Key Mapped User'
        ELSE 'Other'
    END AS principal_category,
    DB_NAME() AS database_name
FROM sys.database_principals p
WHERE p.is_fixed_role = 0 
    AND p.type <> 'X'
    AND p.name NOT IN ('guest', 'INFORMATION_SCHEMA', 'sys')
    AND p.name = ?  -- Parameter for specific principal lookup
ORDER BY p.type_desc, p.name`

// DatabasePrincipalDetailQueryAzureSQL returns the Azure SQL Database specific detail query
const DatabasePrincipalDetailQueryAzureSQL = DatabasePrincipalDetailQuery

// DatabasePrincipalDetailQueryAzureMI returns the Azure SQL Managed Instance specific detail query
const DatabasePrincipalDetailQueryAzureMI = DatabasePrincipalDetailQuery

// DatabasePrincipalRoleMembersQuery returns information about role membership
// This query shows which users belong to which roles for security analysis
const DatabasePrincipalRoleMembersQuery = `SELECT 
    r.name AS role_name,
    r.type_desc AS role_type,
    m.name AS member_name,
    m.type_desc AS member_type,
    DB_NAME() AS database_name
FROM sys.database_role_members rm
    INNER JOIN sys.database_principals r ON rm.role_principal_id = r.principal_id
    INNER JOIN sys.database_principals m ON rm.member_principal_id = m.principal_id
WHERE r.is_fixed_role = 0  -- Only custom roles
    AND m.is_fixed_role = 0  -- Only custom members
ORDER BY r.name, m.name`

// DatabasePrincipalRoleMembersQueryAzureSQL returns the Azure SQL Database specific role members query
const DatabasePrincipalRoleMembersQueryAzureSQL = DatabasePrincipalRoleMembersQuery

// DatabasePrincipalRoleMembersQueryAzureMI returns the Azure SQL Managed Instance specific role members query
const DatabasePrincipalRoleMembersQueryAzureMI = DatabasePrincipalRoleMembersQuery
