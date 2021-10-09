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

import (
	"context"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/option"
)

type Database struct {
	client     *spanner.Client
	databaseID *DatabaseID
}

func (database *Database) Client() *spanner.Client {
	return database.client
}

func (database *Database) DatabaseID() *DatabaseID {
	return database.databaseID
}

func NewDatabase(ctx context.Context, databaseID *DatabaseID, credentialsFilePath string) (*Database, error) {
	credentialsFileClientOption := option.WithCredentialsFile(credentialsFilePath)
	client, err := spanner.NewClient(ctx, databaseID.ID(), credentialsFileClientOption)

	if err != nil {
		return nil, err
	}

	return NewDatabaseFromClient(client, databaseID), nil
}

func NewDatabaseFromClient(client *spanner.Client, databaseID *DatabaseID) *Database {
	return &Database{
		client:     client,
		databaseID: databaseID,
	}
}
