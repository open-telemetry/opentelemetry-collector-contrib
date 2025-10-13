// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the database role membership metrics scraper for SQL Server.
// This file implements comprehensive collection of database role membership information
// to provide visibility into access control structures within databases.
//
// Database Role Membership Scraper Overview:
//
// Database role membership is a critical component of SQL Server security architecture.
// This scraper extracts role-member relationships from system catalog views to provide
// monitoring capabilities for database security auditing and compliance.
//
// Based on User Requirements:
// This implementation uses the provided query for role membership monitoring:
// "SELECT roles.name AS role_name, members.name AS member_name
//
//	FROM sys.database_role_members AS drm
//	JOIN sys.database_principals AS roles ON drm.role_principal_id = roles.principal_id
//	JOIN sys.database_principals AS members ON drm.member_principal_id = members.principal_id
//	ORDER BY role_name, member_name;"
package scrapers

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// DatabaseRoleMembershipScraper implements scraping for database role membership metrics
// This scraper provides comprehensive monitoring of database role-member relationships
// for security auditing and access control visibility
type DatabaseRoleMembershipScraper struct {
	logger        *zap.Logger
	connection    SQLConnectionInterface
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewDatabaseRoleMembershipScraper creates a new scraper for database role membership metrics
// This constructor initializes the scraper with necessary dependencies for metric collection
func NewDatabaseRoleMembershipScraper(logger *zap.Logger, connection SQLConnectionInterface, engineEdition int) *DatabaseRoleMembershipScraper {
	return &DatabaseRoleMembershipScraper{
		connection:    connection,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeDatabaseRoleMembershipMetrics collects individual database role membership metrics using engine-specific queries
func (s *DatabaseRoleMembershipScraper) ScrapeDatabaseRoleMembershipMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database role membership metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("database_role_membership")
	if !found {
		return fmt.Errorf("no database role membership query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database role membership query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseRoleMembershipMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database role membership query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database role membership: %w", err)
	}

	s.logger.Debug("Database role membership query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each role membership result
	for _, result := range results {
		if err := s.processDatabaseRoleMembershipMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database role membership metrics",
				zap.Error(err),
				zap.String("role_name", result.RoleName),
				zap.String("member_name", result.MemberName),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabaseRoleMembershipSummaryMetrics collects aggregated database role membership statistics
func (s *DatabaseRoleMembershipScraper) ScrapeDatabaseRoleMembershipSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database role membership summary metrics")

	// Get the appropriate summary query for this engine edition
	query, found := s.getQueryForMetric("database_role_membership_summary")
	if !found {
		return fmt.Errorf("no database role membership summary query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database role membership summary query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseRoleMembershipSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database role membership summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database role membership summary: %w", err)
	}

	s.logger.Debug("Database role membership summary query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each summary result
	for _, result := range results {
		if err := s.processDatabaseRoleMembershipSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database role membership summary metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabaseRoleHierarchyMetrics collects database role hierarchy and nesting information
func (s *DatabaseRoleMembershipScraper) ScrapeDatabaseRoleHierarchyMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database role hierarchy metrics")

	// Get the appropriate hierarchy query for this engine edition
	query, found := s.getQueryForMetric("database_role_hierarchy")
	if !found {
		return fmt.Errorf("no database role hierarchy query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database role hierarchy query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseRoleHierarchy
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database role hierarchy query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database role hierarchy: %w", err)
	}

	s.logger.Debug("Database role hierarchy query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each hierarchy result
	for _, result := range results {
		if err := s.processDatabaseRoleHierarchyMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database role hierarchy metrics",
				zap.Error(err),
				zap.String("parent_role", result.ParentRoleName),
				zap.String("child_role", result.ChildRoleName),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabaseRoleActivityMetrics collects database role activity and usage metrics
func (s *DatabaseRoleMembershipScraper) ScrapeDatabaseRoleActivityMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database role activity metrics")

	// Get the appropriate activity query for this engine edition
	query, found := s.getQueryForMetric("database_role_activity")
	if !found {
		return fmt.Errorf("no database role activity query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database role activity query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseRoleActivity
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database role activity query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database role activity: %w", err)
	}

	s.logger.Debug("Database role activity query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each activity result
	for _, result := range results {
		if err := s.processDatabaseRoleActivityMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database role activity metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabaseRolePermissionMatrixMetrics collects database role permission analysis
func (s *DatabaseRoleMembershipScraper) ScrapeDatabaseRolePermissionMatrixMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database role permission matrix metrics")

	// Get the appropriate permission matrix query for this engine edition
	query, found := s.getQueryForMetric("database_role_permission_matrix")
	if !found {
		return fmt.Errorf("no database role permission matrix query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database role permission matrix query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseRolePermissionMatrix
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database role permission matrix query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database role permission matrix: %w", err)
	}

	s.logger.Debug("Database role permission matrix query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each permission matrix result
	for _, result := range results {
		if err := s.processDatabaseRolePermissionMatrixMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database role permission matrix metrics",
				zap.Error(err),
				zap.String("role_name", result.RoleName),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition with Default fallback
func (s *DatabaseRoleMembershipScraper) getQueryForMetric(metricName string) (string, bool) {
	query, found := queries.GetQueryForMetric(queries.DatabaseRoleMembershipQueries, metricName, s.engineEdition)
	if found {
		s.logger.Debug("Using query for metric",
			zap.String("metric_name", metricName),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
	}
	return query, found
}

// processDatabaseRoleMembershipMetrics processes individual role membership metrics and adds them to the scope
func (s *DatabaseRoleMembershipScraper) processDatabaseRoleMembershipMetrics(result models.DatabaseRoleMembershipMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Create metric for membership active status
	if result.MembershipActive != nil {
		membershipActiveMetric := scopeMetrics.Metrics().AppendEmpty()
		membershipActiveMetric.SetName("sqlserver.database.role.membership.active")
		membershipActiveMetric.SetDescription("Database role membership active status")

		gauge := membershipActiveMetric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.MembershipActive)

		// Add attributes
		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
		attributes.PutStr("sqlserver.database.role.name", result.RoleName)
		attributes.PutStr("sqlserver.database.role.member.name", result.MemberName)
		attributes.PutStr("sqlserver.database.role.type", result.RoleType)
		attributes.PutStr("sqlserver.database.role.member.type", result.MemberType)
	}

	return nil
}

// processDatabaseRoleMembershipSummaryMetrics processes summary metrics and adds them to the scope
func (s *DatabaseRoleMembershipScraper) processDatabaseRoleMembershipSummaryMetrics(result models.DatabaseRoleMembershipSummary, scopeMetrics pmetric.ScopeMetrics) error {
	// Process each summary metric field
	if result.TotalMemberships != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.memberships.total")
		metric.SetDescription("Total number of role memberships")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.TotalMemberships)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.UniqueRoles != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.roles.withMembers")
		metric.SetDescription("Number of unique roles with members")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.UniqueRoles)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.UniqueMembers != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.members.unique")
		metric.SetDescription("Number of unique members in roles")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.UniqueMembers)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.CustomRoleMemberships != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.memberships.custom")
		metric.SetDescription("Number of custom role memberships")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.CustomRoleMemberships)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.NestedRoleMemberships != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.memberships.nested")
		metric.SetDescription("Number of nested role memberships")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.NestedRoleMemberships)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.UserRoleMemberships != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.memberships.users")
		metric.SetDescription("Number of user role memberships")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.UserRoleMemberships)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	return nil
}

// processDatabaseRoleHierarchyMetrics processes hierarchy metrics and adds them to the scope
func (s *DatabaseRoleMembershipScraper) processDatabaseRoleHierarchyMetrics(result models.DatabaseRoleHierarchy, scopeMetrics pmetric.ScopeMetrics) error {
	if result.NestingLevel != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.nesting.level")
		metric.SetDescription("Role nesting level")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.NestingLevel)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
		attributes.PutStr("sqlserver.database.role.parent.name", result.ParentRoleName)
		attributes.PutStr("sqlserver.database.role.child.name", result.ChildRoleName)
	}

	if result.EffectivePermissions != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.permissions.inherited")
		metric.SetDescription("Role permission inheritance status")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.EffectivePermissions)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
		attributes.PutStr("sqlserver.database.role.parent.name", result.ParentRoleName)
		attributes.PutStr("sqlserver.database.role.child.name", result.ChildRoleName)
	}

	return nil
}

// processDatabaseRoleActivityMetrics processes activity metrics and adds them to the scope
func (s *DatabaseRoleMembershipScraper) processDatabaseRoleActivityMetrics(result models.DatabaseRoleActivity, scopeMetrics pmetric.ScopeMetrics) error {
	if result.ActiveMemberships != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.memberships.active")
		metric.SetDescription("Number of active role memberships")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.ActiveMemberships)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.EmptyRoles != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.roles.empty")
		metric.SetDescription("Number of empty roles")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.EmptyRoles)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.HighPrivilegeMembers != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.members.highPrivilege")
		metric.SetDescription("Number of high privilege role members")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.HighPrivilegeMembers)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.ApplicationRoleMembers != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.members.applicationRoles")
		metric.SetDescription("Number of application role members")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.ApplicationRoleMembers)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	if result.CrossRoleMembers != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.members.crossRole")
		metric.SetDescription("Number of cross-role members")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.CrossRoleMembers)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
	}

	return nil
}

// processDatabaseRolePermissionMatrixMetrics processes permission matrix metrics and adds them to the scope
func (s *DatabaseRoleMembershipScraper) processDatabaseRolePermissionMatrixMetrics(result models.DatabaseRolePermissionMatrix, scopeMetrics pmetric.ScopeMetrics) error {
	if result.MemberCount != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.permission.memberCount")
		metric.SetDescription("Number of members in role")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.MemberCount)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
		attributes.PutStr("sqlserver.database.role.permission.roleName", result.RoleName)
		attributes.PutStr("sqlserver.database.role.permission.scope", result.PermissionScope)
	}

	if result.RiskLevel != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.database.role.permission.riskLevel")
		metric.SetDescription("Role risk level")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.RiskLevel)

		attributes := dataPoint.Attributes()
		attributes.PutStr("sqlserver.database.name", result.DatabaseName)
		attributes.PutStr("sqlserver.database.role.permission.roleName", result.RoleName)
		attributes.PutStr("sqlserver.database.role.permission.scope", result.PermissionScope)
	}

	return nil
}
