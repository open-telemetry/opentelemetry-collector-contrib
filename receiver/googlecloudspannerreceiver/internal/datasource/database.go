// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasource // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"

import (
	"context"
	"os"

	"cloud.google.com/go/spanner"
	"golang.org/x/oauth2/google"
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
	var client *spanner.Client
	var err error

	if credentialsFilePath != "" {
		var credentialsJSON []byte
		credentialsJSON, err = os.ReadFile(credentialsFilePath)
		if err != nil {
			return nil, err
		}
		var creds *google.Credentials
		creds, err = google.CredentialsFromJSONWithType(ctx, credentialsJSON, google.ServiceAccount, spanner.Scope)
		if err != nil {
			return nil, err
		}
		client, err = spanner.NewClient(ctx, databaseID.ID(), option.WithCredentials(creds))
	} else {
		// Fallback to Application Default Credentials(https://google.aip.dev/auth/4110)
		client, err = spanner.NewClient(ctx, databaseID.ID())
	}

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
