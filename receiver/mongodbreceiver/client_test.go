// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.uber.org/zap"
)

// fakeClient is a mocked client of the scraper
// was not able to optimize scraping multiple databases via goroutines
// while also testing with exclusively mtest.
type fakeClient struct{ mock.Mock }

func (fc *fakeClient) ListDatabaseNames(ctx context.Context, filters any, opts ...options.Lister[options.ListDatabasesOptions]) ([]string, error) {
	args := fc.Called(ctx, filters, opts)
	return args.Get(0).([]string), args.Error(1)
}

func (fc *fakeClient) ListCollectionNames(ctx context.Context, dbName string) ([]string, error) {
	args := fc.Called(ctx, dbName)
	return args.Get(0).([]string), args.Error(1)
}

func (fc *fakeClient) Disconnect(ctx context.Context) error {
	args := fc.Called(ctx)
	return args.Error(0)
}

func (fc *fakeClient) Connect(ctx context.Context) error {
	args := fc.Called(ctx)
	return args.Error(0)
}

func (fc *fakeClient) GetVersion(ctx context.Context) (*version.Version, error) {
	args := fc.Called(ctx)
	return args.Get(0).(*version.Version), args.Error(1)
}

func (fc *fakeClient) ServerStatus(ctx context.Context, dbName string) (bson.M, error) {
	args := fc.Called(ctx, dbName)
	return args.Get(0).(bson.M), args.Error(1)
}

func (fc *fakeClient) DBStats(ctx context.Context, dbName string) (bson.M, error) {
	args := fc.Called(ctx, dbName)
	return args.Get(0).(bson.M), args.Error(1)
}

func (fc *fakeClient) TopStats(ctx context.Context) (bson.M, error) {
	args := fc.Called(ctx)
	return args.Get(0).(bson.M), args.Error(1)
}

func (fc *fakeClient) IndexStats(ctx context.Context, dbName, collectionName string) ([]bson.M, error) {
	args := fc.Called(ctx, dbName, collectionName)
	return args.Get(0).([]bson.M), args.Error(1)
}

func (fc *fakeClient) RunCommand(ctx context.Context, db string, command bson.M) (bson.M, error) {
	args := fc.Called(ctx, db, command)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	result, ok := args.Get(0).(bson.M)
	if !ok {
		err := errors.New("mock returned invalid type")
		zap.L().Error("type assertion failed",
			zap.String("expected", "bson.M"))
		return nil, err
	}

	return result, args.Error(1)
}

func TestListDatabaseNames(t *testing.T) {
	mont := drivertest.NewMockDeployment()
	mont.AddResponses(bson.D{
		bson.E{
			Key:   "ok",
			Value: 1,
		},
		bson.E{
			Key: "databases",
			Value: []struct {
				Name string `bson:"name,omitempty"`
			}{
				{Name: "admin"},
			},
		},
	})
	opts := options.Client()
	//nolint:staticcheck // Using deprecated Deployment field for testing purposes
	opts.Deployment = mont
	c, err := mongo.Connect(opts)
	require.NoError(t, err)

	client := &mongodbClient{
		Client: c,
	}
	dbNames, err := client.ListDatabaseNames(context.Background(), bson.D{})
	require.NoError(t, err)
	require.Equal(t, "admin", dbNames[0])
}

type commandString = string

const (
	dbStatsType      commandString = "dbStats"
	serverStatusType commandString = "serverStatus"
	topType          commandString = "top"
)

func TestRunCommands(t *testing.T) {
	loadedDbStats, err := loadDBStats()
	require.NoError(t, err)
	loadedServerStatus, err := loadServerStatus()
	require.NoError(t, err)
	loadedTop, err := loadTop()
	require.NoError(t, err)

	testCases := []struct {
		desc     string
		cmd      commandString
		response bson.D
		validate func(*testing.T, bson.M)
	}{
		{
			desc:     "dbStats success",
			cmd:      dbStatsType,
			response: loadedDbStats,
			validate: func(t *testing.T, m bson.M) {
				require.Equal(t, float64(16384), m["indexSize"])
			},
		},
		{
			desc:     "serverStatus success",
			cmd:      serverStatusType,
			response: loadedServerStatus,
			validate: func(t *testing.T, m bson.M) {
				mem, err := dig(m, []string{"mem", "mapped"})
				require.NoError(t, err)
				require.Equal(t, int32(0), mem)
			},
		},
		{
			desc:     "top success",
			cmd:      topType,
			response: loadedTop,
			validate: func(t *testing.T, m bson.M) {
				commands, err := dig(m, []string{"totals", "local.oplog.rs", "commands", "time"})
				require.NoError(t, err)
				require.Equal(t, int32(540), commands)
			},
		},
	}

	for _, tc := range testCases {
		mont := drivertest.NewMockDeployment()
		mont.AddResponses(tc.response)
		opts := options.Client()
		//nolint:staticcheck // Using deprecated Deployment field for testing purposes
		opts.Deployment = mont
		c, err := mongo.Connect(opts)
		require.NoError(t, err)

		client := &mongodbClient{
			Client: c,
			logger: zap.NewNop(),
		}

		var result bson.M
		switch tc.cmd {
		case serverStatusType:
			result, err = client.ServerStatus(context.Background(), "test")
		case dbStatsType:
			result, err = client.DBStats(context.Background(), "test")
		case topType:
			result, err = client.TopStats(context.Background())
		}
		require.NoError(t, err)
		if tc.validate != nil {
			tc.validate(t, result)
		}
	}
}

func TestGetVersion(t *testing.T) {
	mont := drivertest.NewMockDeployment()

	buildInfo, err := loadBuildInfo()
	require.NoError(t, err)

	mont.AddResponses(buildInfo)

	opts := options.Client()
	//nolint:staticcheck // Using deprecated Deployment field for testing purposes
	opts.Deployment = mont
	c, err := mongo.Connect(opts)
	require.NoError(t, err)

	client := mongodbClient{
		Client: c,
		logger: zap.NewNop(),
	}

	version, err := client.GetVersion(context.TODO())
	require.NoError(t, err)
	require.Equal(t, "4.4.10", version.String())
}

func TestGetVersionFailures(t *testing.T) {
	mt := drivertest.NewMockDeployment()

	malformedBuildInfo := bson.D{
		bson.E{Key: "ok", Value: 1},
		bson.E{Key: "version", Value: 1},
	}

	testCases := []struct {
		desc         string
		responses    []bson.D
		partialError string
	}{
		{
			desc: "Unable to run buildInfo",
			responses: []bson.D{{
				bson.E{Key: "ok", Value: 0},
				bson.E{Key: "code", Value: mongo.CommandError{}.Code},
				bson.E{Key: "errmsg", Value: mongo.CommandError{}.Message},
				bson.E{Key: "codeName", Value: mongo.CommandError{}.Name},
			}},
			partialError: "unable to get build info",
		},
		{
			desc:         "unable to parse version",
			responses:    []bson.D{malformedBuildInfo},
			partialError: "unable to parse mongo version from server",
		},
	}

	for _, tc := range testCases {
		mt.AddResponses(tc.responses...)
		opts := options.Client()
		//nolint:staticcheck // Using deprecated Deployment field for testing purposes
		opts.Deployment = mt
		c, err := mongo.Connect(opts)
		require.NoError(t, err)

		client := mongodbClient{
			Client: c,
			logger: zap.NewNop(),
		}

		_, err = client.GetVersion(context.Background())
		require.ErrorContains(t, err, tc.partialError)
	}
}

func loadDBStats() (bson.D, error) {
	return loadTestFile("./testdata/dbstats.json")
}

func loadDBStatsAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/dbstats.json")
}

func loadServerStatus() (bson.D, error) {
	return loadTestFile("./testdata/serverStatus.json")
}

func loadServerStatusAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/serverStatus.json")
}

func loadTop() (bson.D, error) {
	return loadTestFile("./testdata/top.json")
}

func loadTopAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/top.json")
}

func loadIndexStatsAsMap(collectionName string) ([]bson.M, error) {
	var indexStats []bson.M
	switch collectionName {
	case "products":
		indexStats0, _ := loadTestFileAsMap("./testdata/productsIndexStats0.json")
		indexStats = append(indexStats, indexStats0)
	case "orders":
		indexStats0, _ := loadTestFileAsMap("./testdata/ordersIndexStats0.json")
		indexStats1, _ := loadTestFileAsMap("./testdata/ordersIndexStats1.json")
		indexStats2, _ := loadTestFileAsMap("./testdata/ordersIndexStats2.json")
		indexStats = append(indexStats, indexStats0, indexStats1, indexStats2)
	case "error":
		indexStatsError, _ := loadTestFileAsMap("./testdata/indexStatsError.json")
		indexStats = append(indexStats, indexStatsError)
	default:
		return nil, errors.New("failed to load index stats from an unknown collection name")
	}
	return indexStats, nil
}

func loadBuildInfo() (bson.D, error) {
	return loadTestFile("./testdata/buildInfo.json")
}

func loadAdminStatusAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/admin.json")
}

func loadOnlyStorageEngineAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/only_storage_engine.json")
}

func loadTestFile(filePath string) (bson.D, error) {
	var doc bson.D
	testFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = bson.UnmarshalExtJSON(testFile, true, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func loadTestFileAsMap(filePath string) (bson.M, error) {
	var doc bson.M
	testFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = bson.UnmarshalExtJSON(testFile, true, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
