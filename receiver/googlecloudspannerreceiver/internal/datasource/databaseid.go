// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasource // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"

import "fmt"

type DatabaseID struct {
	projectID    string
	instanceID   string
	databaseName string
	id           string
}

func NewDatabaseID(projectID string, instanceID string, databaseName string) *DatabaseID {
	return &DatabaseID{
		projectID:    projectID,
		instanceID:   instanceID,
		databaseName: databaseName,
		id:           fmt.Sprintf("projects/%v/instances/%v/databases/%v", projectID, instanceID, databaseName),
	}
}

func (databaseID *DatabaseID) ProjectID() string {
	return databaseID.projectID
}

func (databaseID *DatabaseID) InstanceID() string {
	return databaseID.instanceID
}

func (databaseID *DatabaseID) DatabaseName() string {
	return databaseID.databaseName
}
func (databaseID *DatabaseID) ID() string {
	return databaseID.id
}
