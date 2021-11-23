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
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
)

func TestNewDatabaseFromClient(t *testing.T) {
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	databaseID := databaseID()

	database := NewDatabaseFromClient(client, databaseID)

	assert.Equal(t, client, database.Client())
	assert.Equal(t, databaseID, database.DatabaseID())
}

func TestNewDatabase(t *testing.T) {
	ctx := context.Background()
	databaseID := databaseID()

	database, err := NewDatabase(ctx, databaseID, "../../testdata/serviceAccount.json")

	assert.Nil(t, err)
	assert.NotNil(t, database.Client())
	assert.Equal(t, databaseID, database.DatabaseID())
}

func TestNewDatabaseWithError(t *testing.T) {
	ctx := context.Background()
	databaseID := databaseID()

	database, err := NewDatabase(ctx, databaseID, "does not exist")

	assert.NotNil(t, err)
	assert.Nil(t, database)
}
