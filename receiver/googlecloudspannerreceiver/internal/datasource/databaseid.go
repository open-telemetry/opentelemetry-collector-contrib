// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasource

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
