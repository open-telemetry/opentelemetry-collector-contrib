// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	projectID    = "projectID"
	instanceID   = "instanceID"
	databaseName = "DatabaseName"
	databaseRole = "DatabaseRole"
)

func databaseID() *DatabaseID {
	return databaseIDWithDatabaseRole("")
}

func databaseIDWithDatabaseRole(databaseRole string) *DatabaseID {
	return NewDatabaseID(projectID, instanceID, databaseName, databaseRole)
}

func TestNewDatabaseId(t *testing.T) {

	databaseID := databaseIDWithDatabaseRole(databaseRole)

	assert.Equal(t, projectID, databaseID.projectID)
	assert.Equal(t, instanceID, databaseID.instanceID)
	assert.Equal(t, databaseName, databaseID.databaseName)
	assert.Equal(t, databaseRole, databaseID.databaseRole)
	assert.Equal(t, "projects/"+projectID+"/instances/"+instanceID+"/databases/"+databaseName, databaseID.id)
	assert.Equal(t, projectID, databaseID.ProjectID())
	assert.Equal(t, instanceID, databaseID.InstanceID())
	assert.Equal(t, databaseName, databaseID.DatabaseName())
	assert.Equal(t, databaseRole, databaseID.DatabaseRole())
	assert.Equal(t, "projects/"+projectID+"/instances/"+instanceID+"/databases/"+databaseName, databaseID.ID())
}
