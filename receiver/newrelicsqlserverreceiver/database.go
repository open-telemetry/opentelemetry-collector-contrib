// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

const (
	// SQL queries for database operations - based on nri-mssql database package

	// databaseNameQuery gets all database names excluding system databases
	// Based on nri-mssql database.databaseNameQuery
	databaseNameQuery = `SELECT name as db_name 
		FROM sys.databases 
		WHERE name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')`

	// engineEditionQuery gets the SQL Server engine edition
	// Based on nri-mssql database.engineEditionQuery
	engineEditionQuery = "SELECT SERVERPROPERTY('EngineEdition') AS EngineEdition"

	// Engine edition constants based on nri-mssql
	AzureSQLDatabaseEngineEditionNumber        = 5
	AzureSQLManagedInstanceEngineEditionNumber = 8
)

// DatabaseNameRow represents a database name query result
// Based on nri-mssql database.NameRow struct
type DatabaseNameRow struct {
	DBName string `db:"db_name"`
}

// DatabaseInfo represents metadata about a database
type DatabaseInfo struct {
	Name          string
	EngineEdition int
}

// DatabaseHelper provides database-related functionality
// Based on nri-mssql database package functionality
type DatabaseHelper struct {
	connection *SQLConnection
	logger     *zap.Logger
}

// NewDatabaseHelper creates a new database helper instance
func NewDatabaseHelper(connection *SQLConnection, logger *zap.Logger) *DatabaseHelper {
	return &DatabaseHelper{
		connection: connection,
		logger:     logger,
	}
}

// GetDatabaseNames retrieves all user database names from the SQL Server instance
// Based on nri-mssql database.CreateDatabaseEntities functionality
func (dh *DatabaseHelper) GetDatabaseNames(ctx context.Context) ([]string, error) {
	databaseRows := make([]*DatabaseNameRow, 0)
	if err := dh.connection.Query(ctx, &databaseRows, databaseNameQuery); err != nil {
		return nil, fmt.Errorf("failed to query database names: %w", err)
	}

	dbNames := make([]string, len(databaseRows))
	for i, row := range databaseRows {
		dbNames[i] = row.DBName
	}

	dh.logger.Debug("Retrieved database names", zap.Strings("databases", dbNames))
	return dbNames, nil
}

// GetEngineEdition retrieves the SQL Server engine edition
// Based on nri-mssql database.engineEditionQuery functionality
func (dh *DatabaseHelper) GetEngineEdition(ctx context.Context) (int, error) {
	var engineEdition int
	row := dh.connection.QueryRow(ctx, engineEditionQuery)
	if err := row.Scan(&engineEdition); err != nil {
		return 0, fmt.Errorf("failed to get engine edition: %w", err)
	}

	dh.logger.Debug("Retrieved engine edition", zap.Int("edition", engineEdition))
	return engineEdition, nil
}

// IsAzureSQL checks if the SQL Server instance is Azure SQL Database or Managed Instance
func (dh *DatabaseHelper) IsAzureSQL(ctx context.Context) (bool, error) {
	engineEdition, err := dh.GetEngineEdition(ctx)
	if err != nil {
		return false, err
	}

	isAzure := engineEdition == AzureSQLDatabaseEngineEditionNumber ||
		engineEdition == AzureSQLManagedInstanceEngineEditionNumber

	dh.logger.Debug("Checked if Azure SQL", zap.Bool("is_azure", isAzure), zap.Int("engine_edition", engineEdition))
	return isAzure, nil
}

// GetDatabaseInfo retrieves comprehensive database information
func (dh *DatabaseHelper) GetDatabaseInfo(ctx context.Context) ([]DatabaseInfo, error) {
	dbNames, err := dh.GetDatabaseNames(ctx)
	if err != nil {
		return nil, err
	}

	engineEdition, err := dh.GetEngineEdition(ctx)
	if err != nil {
		return nil, err
	}

	dbInfos := make([]DatabaseInfo, len(dbNames))
	for i, name := range dbNames {
		dbInfos[i] = DatabaseInfo{
			Name:          name,
			EngineEdition: engineEdition,
		}
	}

	return dbInfos, nil
}

// ValidateDatabaseAccess checks if the connection can access system views
// This is similar to validation functions in nri-mssql
func (dh *DatabaseHelper) ValidateDatabaseAccess(ctx context.Context) error {
	// Test basic sys.databases access
	_, err := dh.GetDatabaseNames(ctx)
	if err != nil {
		return fmt.Errorf("cannot access sys.databases: %w", err)
	}

	// Test SERVERPROPERTY access
	_, err = dh.GetEngineEdition(ctx)
	if err != nil {
		return fmt.Errorf("cannot access SERVERPROPERTY: %w", err)
	}

	dh.logger.Debug("Database access validation passed")
	return nil
}
