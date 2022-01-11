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

package mongodbreceiver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

// Client is an interface that exposes functionality towards a mongo environment
type Client interface {
	ListDatabaseNames(ctx context.Context, filters interface{}, opts ...*options.ListDatabasesOptions) ([]string, error)
	Disconnect(context.Context) error
	Connect(context.Context) error
	TestConnection(context.Context) (*mongoTestConnectionResponse, error)
	ServerStatus(ctx context.Context, DBName string) (bson.M, error)
	DBStats(ctx context.Context, DBName string) (bson.M, error)
}

// driver is the underlying mongo.Client that Client invokes to actually do actions
// against the mongo instance
type driver interface {
	Database(dbName string, opts ...*options.DatabaseOptions) *mongo.Database
	ListDatabaseNames(ctx context.Context, filters interface{}, opts ...*options.ListDatabasesOptions) ([]string, error)
	Connect(context.Context) error
	Disconnect(context.Context) error
	Ping(ctx context.Context, rp *readpref.ReadPref) error
}

// mongodbClient is a mongodb metric scraper client
type mongodbClient struct {
	// underlying mongo client driver
	driver         driver
	username       string
	password       string
	hosts          []string
	replicaSetName string
	logger         *zap.Logger
	timeout        time.Duration
	tlsConfig      *tls.Config
}

// NewClient creates a new client to connect and query mongo for the
// mongodbreceiver
func NewClient(config *Config, logger *zap.Logger) (Client, error) {
	tlsConfig, err := config.LoadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid tls configuration: %w", err)
	}
	hosts := []string{}
	for _, ep := range config.Hosts {
		hosts = append(hosts, ep.Endpoint)
	}

	return &mongodbClient{
		hosts:          hosts,
		username:       config.Username,
		password:       config.Password,
		replicaSetName: config.ReplicaSet,
		timeout:        config.Timeout,
		logger:         logger,
		tlsConfig:      tlsConfig,
	}, nil
}

// Connect establishes a connection to mongodb instance
func (c *mongodbClient) Connect(ctx context.Context) error {
	if err := c.ensureClient(ctx); err != nil {
		return fmt.Errorf("unable to instantiate mongo client: %w", err)
	}
	return nil
}

// Disconnect closes attempts to close any open connections
func (c *mongodbClient) Disconnect(ctx context.Context) error {
	if c.isConnected(ctx) {
		c.logger.Info(fmt.Sprintf("disconnecting from mongo connection for host(s): %v", c.hosts))
		return c.driver.Disconnect(ctx)
	}
	return nil
}

// ListDatabaseNames gets a list of the database name given a filter
func (c *mongodbClient) ListDatabaseNames(ctx context.Context, filters interface{}, options ...*options.ListDatabasesOptions) ([]string, error) {
	return c.driver.ListDatabaseNames(ctx, filters, options...)
}

// Query executes a query against a database. Relies on connection to be established via `Connect()`
func (c *mongodbClient) RunCommand(ctx context.Context, database string, command bson.M) (bson.M, error) {
	db := c.driver.Database(database)
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

type mongoTestConnectionResponse struct {
	Version string
}

// TestConnection validates the underlying connection of the client and returns a result of the
// version of mongo the client is connected to so adjustments in collection protocol can
// be determined
func (c *mongodbClient) TestConnection(ctx context.Context) (*mongoTestConnectionResponse, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect: %w", err)
	}

	res, err := c.RunCommand(ctx, "admin", bson.M{"buildInfo": 1})
	if err != nil {
		return nil, fmt.Errorf("unable to get build info: %w", err)
	}

	v, ok := res["version"].(string)
	if !ok {
		return nil, errors.New("unable to parse mongo version from server")
	}

	c.logger.Debug(fmt.Sprintf("retrieved version information from mongo server to be: %s", v))
	return &mongoTestConnectionResponse{
		Version: v,
	}, nil
}

func (c *mongodbClient) ensureClient(ctx context.Context) error {
	if !c.isConnected(ctx) {
		driver, err := mongo.Connect(ctx, c.options())
		if err != nil {
			return fmt.Errorf("error creating mongo client: %w", err)
		}

		if err := driver.Ping(ctx, readpref.PrimaryPreferred()); err != nil {
			return fmt.Errorf("could not connect to hosts %v: %w", c.hosts, err)
		}

		c.logger.Info(fmt.Sprintf("Mongo connection established to hosts: %s", strings.Join(c.hosts, ", ")))
		c.driver = driver
	}
	return nil
}

func (c *mongodbClient) options() *options.ClientOptions {
	clientOptions := options.Client()

	if c.tlsConfig != nil {
		clientOptions.SetTLSConfig(c.tlsConfig)
	}

	if c.replicaSetName != "" {
		clientOptions.SetReplicaSet(c.replicaSetName)
	}

	clientOptions.ApplyURI(c.connectionString())
	if c.timeout != 0 {
		clientOptions.SetConnectTimeout(c.timeout)
	}

	return clientOptions
}

func (c *mongodbClient) isConnected(ctx context.Context) bool {
	if c.driver == nil {
		return false
	}

	if err := c.driver.Ping(ctx, nil); err != nil {
		return false
	}

	return true
}

// for a full list of options see: https://docs.mongodb.com/manual/reference/connection-string/#connections-connection-options
func (c *mongodbClient) connectionString() string {
	return fmt.Sprintf("mongodb://%s%s",
		authenticationString(c.username, c.password),
		strings.Join(c.hosts, ","))
}

// if username based authentication is configured we should add this to connection string
// https://docs.mongodb.com/manual/reference/connection-string/#components
func authenticationString(username, password string) string {
	if username != "" && password != "" {
		return fmt.Sprintf("%s:%s@", username, password)
	}
	return ""
}
