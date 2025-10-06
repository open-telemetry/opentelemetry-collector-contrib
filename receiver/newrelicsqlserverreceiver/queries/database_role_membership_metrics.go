// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL queries for collecting SQL Server database role membership metrics.
// This file implements comprehensive collection of database role membership information
// showing relationships between roles and their members across different SQL Server engine types.
//
// Database Role Membership Queries Overview:
//
// Database role membership is fundamental to SQL Server security architecture.
// These queries extract role-member relationships from the system catalog views
// to provide visibility into access control structures within databases.
//
// Core Query Logic:
// The base query provided by the user:
// ```sql
// SELECT roles.name AS role_name, members.name AS member_name
// FROM sys.database_role_members AS drm
// JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
// JOIN sys.database_principals AS members ON drm.member_principal_id = members.principal_id
// ORDER BY role_name, member_name;
// ```
package queries

// DatabaseRoleMembershipMetricsQuery returns the main query for role membership relationships
// This implements the core user-provided query with additional context and type information
const DatabaseRoleMembershipMetricsQuery = `
	SELECT 
		roles.name AS role_name,
		members.name AS member_name,
		DB_NAME() AS database_name,
		roles.type_desc AS role_type,
		members.type_desc AS member_type,
		1 AS membership_active
	FROM sys.database_role_members AS drm
	JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
	JOIN sys.database_principals AS members ON drm.member_principal_id = members.principal_id
	WHERE roles.type IN ('R', 'A')  -- DATABASE_ROLE, APPLICATION_ROLE
	AND (members.type <> 'R' OR (members.type = 'R' AND members.name NOT LIKE 'db_%'))  -- Include nested custom roles
	ORDER BY role_name, member_name`

// DatabaseRoleMembershipSummaryQuery returns aggregated role membership statistics
// This provides summary metrics for monitoring and alerting purposes
const DatabaseRoleMembershipSummaryQuery = `
	SELECT 
		DB_NAME() AS database_name,
		COUNT(*) AS total_memberships,
		COUNT(DISTINCT drm.role_principal_id) AS unique_roles,
		COUNT(DISTINCT drm.member_principal_id) AS unique_members,
		SUM(CASE WHEN roles.is_fixed_role = 0 THEN 1 ELSE 0 END) AS custom_role_memberships,
		SUM(CASE WHEN members.type = 'R' THEN 1 ELSE 0 END) AS nested_role_memberships,
		SUM(CASE WHEN members.type IN ('S', 'U', 'G') THEN 1 ELSE 0 END) AS user_role_memberships
	FROM sys.database_role_members AS drm
	JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
	JOIN sys.database_principals AS members ON drm.member_principal_id = members.principal_id
	WHERE roles.type IN ('R', 'A')`

// DatabaseRoleHierarchyQuery returns role hierarchy and nesting information
// This identifies nested role relationships for security analysis
const DatabaseRoleHierarchyQuery = `
	WITH RoleHierarchy AS (
		SELECT 
			DB_NAME() AS database_name,
			roles.name AS parent_role_name,
			members.name AS child_role_name,
			1 AS nesting_level,
			1 AS effective_permissions
		FROM sys.database_role_members AS drm
		JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
		JOIN sys.database_principals AS members ON drm.member_principal_id = members.principal_id
		WHERE roles.type = 'R' AND members.type = 'R'  -- Role to role memberships only
		
		UNION ALL
		
		SELECT 
			rh.database_name,
			rh.parent_role_name,
			newmembers.name AS child_role_name,
			rh.nesting_level + 1,
			1 AS effective_permissions
		FROM RoleHierarchy rh
		JOIN sys.database_principals AS intermediate ON rh.child_role_name = intermediate.name
		JOIN sys.database_role_members AS newdrm ON intermediate.principal_id = newdrm.role_principal_id
		JOIN sys.database_principals AS newmembers ON newdrm.member_principal_id = newmembers.principal_id
		WHERE newmembers.type = 'R' AND rh.nesting_level < 10  -- Prevent infinite recursion
	)
	SELECT * FROM RoleHierarchy`

// DatabaseRoleActivityQuery returns role membership activity metrics
// This provides insights into role usage patterns and potential security issues
const DatabaseRoleActivityQuery = `
	SELECT 
		DB_NAME() AS database_name,
		(SELECT COUNT(*) FROM sys.database_role_members) AS active_memberships,
		(SELECT COUNT(*) FROM sys.database_principals r 
		 WHERE r.type IN ('R', 'A') 
		 AND NOT EXISTS (SELECT 1 FROM sys.database_role_members drm WHERE drm.role_principal_id = r.principal_id)) AS empty_roles,
		(SELECT COUNT(*) FROM sys.database_role_members drm 
		 JOIN sys.database_principals r ON drm.role_principal_id = r.principal_id 
		 WHERE r.name IN ('db_owner', 'db_securityadmin', 'db_accessadmin', 'db_backupoperator', 'db_ddladmin')) AS high_privilege_members,
		(SELECT COUNT(*) FROM sys.database_role_members drm 
		 JOIN sys.database_principals r ON drm.role_principal_id = r.principal_id 
		 WHERE r.type = 'A') AS app_role_members,
		(SELECT COUNT(DISTINCT drm1.member_principal_id) FROM sys.database_role_members drm1 
		 WHERE EXISTS (SELECT 1 FROM sys.database_role_members drm2 
		              WHERE drm2.member_principal_id = drm1.member_principal_id 
		              AND drm2.role_principal_id != drm1.role_principal_id)) AS cross_role_members`

// DatabaseRolePermissionMatrixQuery returns role-based permission analysis
// This provides insights into the permission structure and risk assessment
const DatabaseRolePermissionMatrixQuery = `
	SELECT 
		DB_NAME() AS database_name,
		roles.name AS role_name,
		COUNT(drm.member_principal_id) AS member_count,
		CASE 
			WHEN roles.name LIKE '%read%' OR roles.name LIKE '%select%' THEN 'READ'
			WHEN roles.name LIKE '%write%' OR roles.name LIKE '%insert%' OR roles.name LIKE '%update%' THEN 'WRITE'
			WHEN roles.name IN ('db_owner', 'db_securityadmin') THEN 'ADMIN'
			ELSE 'MIXED'
		END AS permission_scope,
		CASE 
			WHEN roles.name = 'db_owner' THEN 4
			WHEN roles.name IN ('db_securityadmin', 'db_accessadmin') THEN 3
			WHEN roles.name IN ('db_ddladmin', 'db_backupoperator') THEN 2
			ELSE 1
		END AS risk_level,
		NULL AS last_access_time  -- Would require additional tracking
	FROM sys.database_role_members AS drm
	RIGHT JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
	WHERE roles.type IN ('R', 'A')
	GROUP BY roles.name, roles.principal_id
	ORDER BY risk_level DESC, member_count DESC`
