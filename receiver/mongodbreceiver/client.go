// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// client is an interface that exposes functionality towards a mongo environment
type client interface {
	ListDatabaseNames(ctx context.Context, filters any, opts ...*options.ListDatabasesOptions) ([]string, error)
	ListCollectionNames(ctx context.Context, DBName string) ([]string, error)
	Disconnect(context.Context) error
	GetVersion(context.Context) (*version.Version, error)
	GetReplicationInfo(context.Context) (bson.M, error)
	GetFsyncLockInfo(context.Context) (bson.M, error)
	ReplSetStatus(context.Context) (bson.M, error)
	ReplSetConfig(context.Context) (bson.M, error)
	ServerStatus(ctx context.Context, DBName string) (bson.M, error)
	DBStats(ctx context.Context, DBName string) (bson.M, error)
	TopStats(ctx context.Context) (bson.M, error)
	IndexStats(ctx context.Context, DBName, collectionName string) ([]bson.M, error)
	JumboStats(ctx context.Context, DBName string) (bson.M, error)
	CollectionStats(ctx context.Context, DBName, collectionName string) (bson.M, error)
	ConnPoolStats(ctx context.Context, DBName string) (bson.M, error)
}

// mongodbClient is a mongodb metric scraper client
type mongodbClient struct {
	cfg    *Config
	logger *zap.Logger
	*mongo.Client
}

// newClient creates a new client to connect and query mongo for the
// mongodbreceiver
func newClient(ctx context.Context, config *Config, logger *zap.Logger) (client, error) {
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
	return c.Database(database).ListCollectionNames(ctx, bson.D{}, lcOpts)
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

// CollectionStats returns the collection stats per collection for a given database
// more information can be found here: https://www.mongodb.com/docs/manual/reference/operator/aggregation/collStats/
func (c *mongodbClient) CollectionStats(ctx context.Context, database, collectionName string) (bson.M, error) {
	db := c.Client.Database(database)
	collection := db.Collection(collectionName)
	cursor, err := collection.Aggregate(context.Background(), mongo.Pipeline{
		{{"$collStats", bson.D{{"storageStats", bson.D{}}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var collectionStats []bson.M
	if err = cursor.All(context.Background(), &collectionStats); err != nil {
		return nil, err
	}
	return collectionStats[0], nil
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

// ReplicationInfo
type ReplicationInfo struct {
	LogSizeMB  float64 `bson:"logSizeMb"`
	UsedSizeMB float64 `bson:"usedSizeMb"`
	TimeDiff   int64   `bson:"timeDiff"`
}

// GetReplicationInfo returns same as db.getReplicationInfo() stats using local database
func (c *mongodbClient) GetReplicationInfo(ctx context.Context) (bson.M, error) {
	localdb := c.Database("local")
	collectionNames := []string{"oplog.rs", "oplog.$main"}
	var oplogCollection *mongo.Collection

	for _, name := range collectionNames {
		collection := localdb.Collection(name)
		_, err := collection.EstimatedDocumentCount(ctx)
		if err == nil {
			oplogCollection = collection
			break
		}
	}

	if oplogCollection == nil {
		return nil, fmt.Errorf("unable to find oplog collection")
	}

	// Get oplog collection stats
	collStats := bson.M{}
	err := localdb.RunCommand(ctx, bson.D{{"collStats", oplogCollection.Name()}}).Decode(&collStats)
	if err != nil {
		return nil, fmt.Errorf("unable to get collection stats: %w", err)
	}
	replicationInfo := &ReplicationInfo{}
	if size, ok := collStats["size"].(int32); ok {
		replicationInfo.UsedSizeMB = float64(size) / float64(1024*1024)
	}

	if cappedSize, ok := collStats["maxSize"].(int64); ok {
		replicationInfo.LogSizeMB = float64(cappedSize) / (1024 * 1024)
	} else if cappedSize, ok := collStats["storageSize"].(int64); ok {
		replicationInfo.LogSizeMB = float64(cappedSize) / (1024 * 1024)
	} else {
		return nil, fmt.Errorf("unable to determine the oplog size")
	}

	// Get time difference between first and last oplog entry
	firstEntry := bson.M{}
	lastEntry := bson.M{}

	err = oplogCollection.FindOne(ctx, bson.M{"ts": bson.M{"$exists": true}}, options.FindOne().SetSort(bson.D{{"$natural", 1}})).Decode(&firstEntry)
	if err != nil {
		return nil, fmt.Errorf("unable to get first oplog entry: %w", err)
	}

	err = oplogCollection.FindOne(ctx, bson.M{"ts": bson.M{"$exists": true}}, options.FindOne().SetSort(bson.D{{"$natural", -1}})).Decode(&lastEntry)
	if err != nil {
		return nil, fmt.Errorf("unable to get last oplog entry: %w", err)
	}

	firstTimestamp, firstOk := firstEntry["ts"].(primitive.Timestamp)
	lastTimestamp, lastOk := lastEntry["ts"].(primitive.Timestamp)

	if firstOk && lastOk {
		firstTime := time.Unix(int64(firstTimestamp.T), 0)
		lastTime := time.Unix(int64(lastTimestamp.T), 0)
		timeDiff := lastTime.Sub(firstTime).Seconds()
		replicationInfo.TimeDiff = int64(timeDiff)
	}

	// Convert struct to BSON bytes
	bsonBytes, err := bson.Marshal(replicationInfo)
	if err != nil {
		return nil, fmt.Errorf("error marshaling struct: %w", err)
	}

	// Unmarshal BSON bytes to bson.M
	var bsonMap bson.M
	err = bson.Unmarshal(bsonBytes, &bsonMap)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling to bson.M: %w", err)
	}

	return bsonMap, nil
}

// JumboStats returns the total and jumbo chunk stats for all collections within the specified database
func (c *mongodbClient) JumboStats(ctx context.Context, database string) (bson.M, error) {
	db := c.Client.Database(database)
	total_chunks_res := db.RunCommand(ctx, bson.D{{"count", "chunks"}, {"query", bson.D{}}})
	jumbo_chunks_res := db.RunCommand(ctx, bson.D{{"count", "chunks"}, {"query", bson.D{{"jumbo", true}}}})

	var total_chunks, jumbo_chunks bson.M
	total_chunks_res.Decode(&total_chunks)
	jumbo_chunks_res.Decode(&jumbo_chunks)

	result := bson.M{
		"total": total_chunks["n"],
		"jumbo": jumbo_chunks["n"],
	}
	return result, nil
}

// ConnPoolStats returns the result of db.runCommand({ connPoolStats: 1 })
// more information can be found here: https://docs.mongodb.com/manual/reference/command/connPoolStats/
func (c *mongodbClient) ConnPoolStats(ctx context.Context, database string) (bson.M, error) {
	return c.RunCommand(ctx, database, bson.M{"connPoolStats": 1})
}

type ReplSetStatus struct {
	SetName string   `bson:"set"`
	Members []Member `bson:"members"`
}

type Member struct {
	Id             int       `bson:"_id"`
	Name           string    `bson:"name"`
	State          int       `bson:"state"`
	StateStr       string    `bson:"stateStr"`
	Health         int       `bson:"health"`
	OptimeDate     time.Time `bson:"optimeDate"`
	Self           bool      `bson:"self"`
	OptimeLag      int       `bson:"optimeLag"`
	ReplicationLag int       `bson:"replicationLag"`
}

func (c *mongodbClient) ReplSetStatus(ctx context.Context) (bson.M, error) {
	database := "admin"

	var status *ReplSetStatus
	db := c.Database(database)
	err := db.RunCommand(ctx, bson.M{"replSetGetStatus": 1}).Decode(&status)
	if err != nil {
		return nil, fmt.Errorf("unable to get repl set status: %w", err)
	}

	var primary *Member
	var optimeLag float64

	for _, member := range status.Members {
		if member.State == 1 {
			primary = &member
		}
	}

	if primary == nil {
		return nil, fmt.Errorf("primary not found in replica set status: %w", err)
	}

	optimeLag = 0.0

	for _, member := range status.Members {
		if member.State != 1 && !primary.OptimeDate.IsZero() && !member.OptimeDate.IsZero() {
			lag := primary.OptimeDate.Sub(member.OptimeDate).Seconds()
			member.ReplicationLag = int(lag)

			// Update max optime lag if this lag is greater
			if lag > optimeLag {
				optimeLag = lag
			}
		}
	}
	primary.OptimeLag = int(optimeLag)

	// Convert struct to BSON bytes
	bsonBytes, err := bson.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("error marshaling struct: %w", err)
	}

	// Unmarshal BSON bytes to bson.M
	var bsonMap bson.M
	err = bson.Unmarshal(bsonBytes, &bsonMap)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling to bson.M: %w", err)
	}
	return bsonMap, nil
}

type ReplSetConfig struct {
	Cfg Cfg `bson:"config"`
}

type Cfg struct {
	SetName string       `bson:"_id"`
	Members []*CfgMember `bson:"members"`
}
type CfgMember struct {
	Id           int     `bson:"_id"`
	Name         string  `bson:"host"`
	Votes        int     `bson:"votes"`
	VoteFraction float64 `bson:"voteFraction"`
}

func (c *mongodbClient) ReplSetConfig(ctx context.Context) (bson.M, error) {
	database := "admin"

	var (
		config     *ReplSetConfig
		totalVotes int
	)

	db := c.Database(database)
	err := db.RunCommand(ctx, bson.M{"replSetGetConfig": 1}).Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to get repl set get config: %w", err)
	}
	for _, member := range config.Cfg.Members {
		totalVotes += member.Votes
	}

	for _, member := range config.Cfg.Members {
		member.VoteFraction = (float64(member.Votes) / float64(totalVotes))
	}

	// Convert struct to BSON bytes
	bsonBytes, err := bson.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("error marshaling struct: %w", err)
	}

	// Unmarshal BSON bytes to bson.M
	var bsonMap bson.M
	err = bson.Unmarshal(bsonBytes, &bsonMap)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling to bson.M: %w", err)
	}
	return bsonMap, nil
}

// GetFsyncLockInfo returns fsynclocked status using admin database
func (c *mongodbClient) GetFsyncLockInfo(ctx context.Context) (bson.M, error) {
	localdb := c.Database("admin")

	// Get admin stats
	adminStats := bson.M{}
	err := localdb.RunCommand(ctx, bson.D{{"currentOp", 1}}).Decode(&adminStats)
	if err != nil {
		return nil, fmt.Errorf("unable to get fsynclock info stats: %w", err)
	}

	fsynclockinfo := bson.M{}
	fsyncLock, ok := adminStats["fsyncLock"]
	if ok && fsyncLock.(bool) {
		fsynclockinfo["fsyncLocked"] = 1
	} else {
		fsynclockinfo["fsyncLocked"] = 0
	}

	return fsynclockinfo, nil
}
