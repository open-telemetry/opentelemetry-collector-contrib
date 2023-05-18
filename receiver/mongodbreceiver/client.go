// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// client is an interface that exposes functionality towards a mongo environment
type client interface {
	ListDatabaseNames(ctx context.Context, filters interface{}, opts ...*options.ListDatabasesOptions) ([]string, error)
	ListCollectionNames(ctx context.Context, DBName string) ([]string, error)
	Disconnect(context.Context) error
	GetVersion(context.Context) (*version.Version, error)
	ServerStatus(ctx context.Context, DBName string) (bson.M, error)
	DBStats(ctx context.Context, DBName string) (bson.M, error)
	TopStats(ctx context.Context) (bson.M, error)
	IndexStats(ctx context.Context, DBName, collectionName string) ([]bson.M, error)
}

// mongodbClient is a mongodb metric scraper client
type mongodbClient struct {
	cfg    *Config
	logger *zap.Logger
	*mongo.Client
}

// NewClient creates a new client to connect and query mongo for the
// mongodbreceiver
func NewClient(ctx context.Context, config *Config, logger *zap.Logger) (client, error) {
	driver, err := mongo.Connect(ctx, config.ClientOptions())
	if err != nil {
		return nil, err
	}

	return &mongodbClient{
		cfg:    config,
		logger: logger,
		Client: driver,
	}, nil
}

// RunCommand executes a query against a database. Relies on connection to be established via `Connect()`
func (c *mongodbClient) RunCommand(ctx context.Context, database string, command bson.M) (bson.M, error) {
	db := c.Database(database)
	result := db.RunCommand(ctx, command)

	var document bson.M
	err := result.Decode(&document)
	return document, err
}

// ServerStatus returns the result of db.runCommand({ serverStatus: 1 })
// more information can be found here: https://docs.mongodb.com/manual/reference/command/serverStatus/
func (c *mongodbClient) ServerStatus(ctx context.Context, database string) (bson.M, error) {
	return c.RunCommand(ctx, database, bson.M{"serverStatus": 1})
}

// DBStats returns the result of db.runCommand({ dbStats: 1 })
// more information can be found here: https://docs.mongodb.com/manual/reference/command/dbStats/
func (c *mongodbClient) DBStats(ctx context.Context, database string) (bson.M, error) {
	return c.RunCommand(ctx, database, bson.M{"dbStats": 1})
}

// TopStats is an admin command that return the result of db.adminCommand({ top: 1 })
// more information can be found here: https://www.mongodb.com/docs/manual/reference/command/top/
func (c *mongodbClient) TopStats(ctx context.Context) (bson.M, error) {
	return c.RunCommand(ctx, "admin", bson.M{"top": 1})
}

// ListCollectionNames returns a list of collection names for a given database
// SetAuthorizedCollections allows a user without the required privilege to run the command ListCollections.
// more information can be found here: https://pkg.go.dev/go.mongodb.org/mongo-driver@v1.9.0/mongo#Database.ListCollectionNames
func (c *mongodbClient) ListCollectionNames(ctx context.Context, database string) ([]string, error) {
	lcOpts := options.ListCollections().SetAuthorizedCollections(true)
	return c.Database(database).ListCollectionNames(context.Background(), bson.D{}, lcOpts)
}

// IndexStats returns the index stats per collection for a given database
// more information can be found here: https://www.mongodb.com/docs/manual/reference/operator/aggregation/indexStats/
func (c *mongodbClient) IndexStats(ctx context.Context, database, collectionName string) ([]bson.M, error) {
	db := c.Client.Database(database)
	collection := db.Collection(collectionName)
	cursor, err := collection.Aggregate(context.Background(), mongo.Pipeline{bson.D{primitive.E{Key: "$indexStats", Value: bson.M{}}}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var indexStats []bson.M
	if err = cursor.All(context.Background(), &indexStats); err != nil {
		return nil, err
	}
	return indexStats, nil
}

// GetVersion returns a result of the version of mongo the client is connected to so adjustments in collection protocol can
// be determined
func (c *mongodbClient) GetVersion(ctx context.Context) (*version.Version, error) {
	res, err := c.RunCommand(ctx, "admin", bson.M{"buildInfo": 1})
	if err != nil {
		return nil, fmt.Errorf("unable to get build info: %w", err)
	}

	v, ok := res["version"].(string)
	if !ok {
		return nil, errors.New("unable to parse mongo version from server")
	}

	return version.NewVersion(v)
}
