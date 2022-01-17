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

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

// client is an interface that exposes functionality towards a mongo environment
type client interface {
	ListDatabaseNames(ctx context.Context, filters interface{}, opts ...*options.ListDatabasesOptions) ([]string, error)
	Disconnect(context.Context) error
	Connect(context.Context) error
	GetVersion(context.Context) (*string, error)
	ServerStatus(ctx context.Context, DBName string) (bson.M, error)
	DBStats(ctx context.Context, DBName string) (bson.M, error)
}

// mongodbClient is a mongodb metric scraper client
type mongodbClient struct {
	cfg    *Config
	logger *zap.Logger
	*mongo.Client
}

// NewClient creates a new client to connect and query mongo for the
// mongodbreceiver
func NewClient(config *Config, logger *zap.Logger) (client, error) {
	return &mongodbClient{
		cfg:    config,
		logger: logger,
	}, nil
}

// Connect establishes a connection to mongodb instance
func (c *mongodbClient) Connect(ctx context.Context) error {
	if c.Client != nil {
		return nil
	}

	driver, err := mongo.Connect(ctx, c.cfg.ClientOptions())
	if err != nil {
		return fmt.Errorf("error creating mongo client: %w", err)
	}

	if err := driver.Ping(ctx, readpref.PrimaryPreferred()); err != nil {
		return fmt.Errorf("could not connect to hosts %v: %w", c.cfg.hostlist(), err)
	}

	c.logger.Info(fmt.Sprintf("Mongo connection established to hosts: %s", strings.Join(c.cfg.hostlist(), ", ")))
	c.Client = driver

	return nil
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

// GetVersion returns a result of the version of mongo the client is connected to so adjustments in collection protocol can
// be determined
func (c *mongodbClient) GetVersion(ctx context.Context) (*string, error) {
	res, err := c.RunCommand(ctx, "admin", bson.M{"buildInfo": 1})
	if err != nil {
		return nil, fmt.Errorf("unable to get build info: %w", err)
	}

	v, ok := res["version"].(string)
	if !ok {
		return nil, errors.New("unable to parse mongo version from server")
	}

	c.logger.Debug(fmt.Sprintf("detected mongo server to be running version: %s", v))
	return &v, nil
}
