// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestNewDatabaseWithNoCredentialsFilePath(t *testing.T) {
	ctx := context.Background()
	databaseID := databaseID()

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "../../testdata/serviceAccount.json")

	database, err := NewDatabase(ctx, databaseID, "")

	assert.Nil(t, err)
	assert.NotNil(t, database.Client())
	assert.Equal(t, databaseID, database.DatabaseID())
}
