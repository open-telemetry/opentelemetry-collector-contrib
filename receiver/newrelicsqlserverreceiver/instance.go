// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/zap"
)

const (
	// instanceNameQuery gets the instance name
	// Based on nri-mssql instance.instanceNameQuery
	instanceNameQuery = "SELECT COALESCE(@@SERVERNAME, SERVERPROPERTY('ServerName'), SERVERPROPERTY('MachineName')) AS instance_name"
)

// InstanceNameRow represents an instance name query result
// Based on nri-mssql instance.NameRow struct
type InstanceNameRow struct {
	Name sql.NullString `db:"instance_name"`
}

// InstanceInfo represents metadata about a SQL Server instance
type InstanceInfo struct {
	Name string
	Host string
}

// InstanceHelper provides instance-related functionality
// Based on nri-mssql instance package functionality
type InstanceHelper struct {
	connection *SQLConnection
	logger     *zap.Logger
}

// NewInstanceHelper creates a new instance helper instance
func NewInstanceHelper(connection *SQLConnection, logger *zap.Logger) *InstanceHelper {
	return &InstanceHelper{
		connection: connection,
		logger:     logger,
	}
}

// GetInstanceName retrieves the SQL Server instance name
// Based on nri-mssql instance.CreateInstanceEntity functionality
func (ih *InstanceHelper) GetInstanceName(ctx context.Context) (string, error) {
	instanceRows := make([]*InstanceNameRow, 0)
	if err := ih.connection.Query(ctx, &instanceRows, instanceNameQuery); err != nil {
		return "", fmt.Errorf("failed to query instance name: %w", err)
	}

	if len(instanceRows) != 1 {
		return "", fmt.Errorf("expected 1 row for instance name, got %d", len(instanceRows))
	}

	instanceName := ih.connection.Host // Default fallback
	if instanceRows[0].Name.Valid {
		instanceName = instanceRows[0].Name.String
	}

	ih.logger.Debug("Retrieved instance name", zap.String("instance_name", instanceName))
	return instanceName, nil
}

// GetInstanceInfo retrieves comprehensive instance information
func (ih *InstanceHelper) GetInstanceInfo(ctx context.Context) (InstanceInfo, error) {
	instanceName, err := ih.GetInstanceName(ctx)
	if err != nil {
		return InstanceInfo{}, err
	}

	return InstanceInfo{
		Name: instanceName,
		Host: ih.connection.Host,
	}, nil
}

// ValidateInstanceAccess checks if the connection can access instance-level information
func (ih *InstanceHelper) ValidateInstanceAccess(ctx context.Context) error {
	_, err := ih.GetInstanceName(ctx)
	if err != nil {
		return fmt.Errorf("cannot access instance information: %w", err)
	}

	ih.logger.Debug("Instance access validation passed")
	return nil
}
