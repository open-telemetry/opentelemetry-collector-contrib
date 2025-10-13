// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data models for SQL Server database principals metrics.
// This file implements comprehensive collection of database security principals
// including users, roles, and security-related metadata.
//
// Database Principals Metrics Overview:
//
// Database principals represent security entities within a SQL Server database that
// can be granted permissions to access database objects. This includes:
//
// 1. Database Users: Entities that can connect to and access the database
//   - SQL Users: Users authenticated by SQL Server
//   - Windows Users: Users authenticated by Windows/Active Directory
//   - Contained Users: Users contained within the database (SQL Server 2012+)
//   - Certificate Users: Users mapped to certificates
//   - Asymmetric Key Users: Users mapped to asymmetric keys
//
// 2. Database Roles: Security principals that group users for permission management
//   - User-defined Roles: Custom roles created for specific purposes
//   - Application Roles: Special roles activated by applications
//   - Fixed Database Roles: Built-in roles with predefined permissions (excluded)
//
// 3. Principal Metadata: Important information about each principal
//   - Principal Name: The name of the user or role
//   - Principal Type: The type of principal (USER, ROLE, etc.)
//   - Creation Date: When the principal was created
//   - Last Modified: When the principal was last modified (if available)
//
// Query Source:
// - sys.database_principals: Contains information about all database-level principals
// - Excludes fixed roles (is_fixed_role = 0) and extended properties users (type <> 'X')
// - Provides comprehensive view of custom security configuration
//
// Metric Purpose:
// - Security auditing and compliance monitoring
// - Database access control visibility
// - User and role lifecycle tracking
// - Security configuration change detection
//
// Usage in Monitoring:
// - Count of different principal types per database
// - Tracking creation and modification of security principals
// - Identifying unused or stale user accounts
// - Monitoring role assignments and permissions structure
//
// Engine Compatibility:
// - Standard SQL Server: Full access to all principal types and metadata
// - Azure SQL Database: Full access within database scope
// - Azure SQL Managed Instance: Full access with enterprise features
package models

import "time"

// DatabasePrincipalsMetrics represents database security principals metrics
// This model captures information about database users, roles, and other security principals
// as defined by the sys.database_principals system catalog view
type DatabasePrincipalsMetrics struct {
	// PrincipalName is the name of the database principal (user, role, etc.)
	// This corresponds to the 'name' column in sys.database_principals
	// Examples: "dbo", "db_owner", "MyCustomUser", "MyApplicationRole"
	PrincipalName string `db:"principal_name" metric_name:"sqlserver.database.principal.name" source_type:"attribute"`

	// TypeDesc describes the type of the principal
	// This corresponds to the 'type_desc' column in sys.database_principals
	// Common values: "SQL_USER", "WINDOWS_USER", "DATABASE_ROLE", "APPLICATION_ROLE",
	//               "CERTIFICATE_MAPPED_USER", "ASYMMETRIC_KEY_MAPPED_USER"
	TypeDesc string `db:"type_desc" metric_name:"sqlserver.database.principal.type" source_type:"attribute"`

	// CreateDate is when the principal was created
	// This corresponds to the 'create_date' column in sys.database_principals
	// Useful for tracking principal lifecycle and security auditing
	CreateDate *time.Time `db:"create_date" metric_name:"sqlserver.database.principal.createDate" source_type:"gauge"`

	// DatabaseName is the name of the database containing this principal
	// This is added as context since principals are database-scoped
	DatabaseName string `db:"database_name" metric_name:"sqlserver.database.name" source_type:"attribute"`
}

// DatabasePrincipalsSummary represents aggregated statistics about database principals
// This model provides summary metrics for monitoring and alerting on principal counts
type DatabasePrincipalsSummary struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"database_name"`

	// TotalPrincipals is the total count of non-fixed role principals in the database
	// Excludes system fixed roles and extended properties users
	TotalPrincipals *int64 `db:"total_principals" metric_name:"sqlserver.database.principals.total" source_type:"gauge"`

	// UserCount is the count of database users (all user types)
	// Includes SQL_USER, WINDOWS_USER, CERTIFICATE_MAPPED_USER, etc.
	UserCount *int64 `db:"user_count" metric_name:"sqlserver.database.principals.users" source_type:"gauge"`

	// RoleCount is the count of custom database roles
	// Includes DATABASE_ROLE and APPLICATION_ROLE, excludes fixed roles
	RoleCount *int64 `db:"role_count" metric_name:"sqlserver.database.principals.roles" source_type:"gauge"`

	// SQLUserCount is the count of SQL Server authenticated users
	// Subset of UserCount, specifically SQL_USER type principals
	SQLUserCount *int64 `db:"sql_user_count" metric_name:"sqlserver.database.principals.sqlUsers" source_type:"gauge"`

	// WindowsUserCount is the count of Windows authenticated users
	// Subset of UserCount, specifically WINDOWS_USER type principals
	WindowsUserCount *int64 `db:"windows_user_count" metric_name:"sqlserver.database.principals.windowsUsers" source_type:"gauge"`

	// ApplicationRoleCount is the count of application roles
	// Subset of RoleCount, specifically APPLICATION_ROLE type principals
	ApplicationRoleCount *int64 `db:"app_role_count" metric_name:"sqlserver.database.principals.applicationRoles" source_type:"gauge"`
}

// DatabasePrincipalActivity represents recent principal activity metrics
// This model tracks changes and activity related to database principals
type DatabasePrincipalActivity struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"database_name"`

	// RecentPrincipals is the count of principals created in the last 30 days
	RecentPrincipals *int64 `db:"recent_principals" metric_name:"sqlserver.database.principals.recentlyCreated" source_type:"gauge"`

	// OldPrincipals is the count of principals created more than 1 year ago
	// Useful for identifying potentially stale accounts
	OldPrincipals *int64 `db:"old_principals" metric_name:"sqlserver.database.principals.old" source_type:"gauge"`

	// OrphanedUsers is the count of users without corresponding server logins
	// Important for security auditing in SQL Server environments
	OrphanedUsers *int64 `db:"orphaned_users" metric_name:"sqlserver.database.principals.orphanedUsers" source_type:"gauge"`
}
