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
	"io/ioutil"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// fakeClient is a mocked client of the scraper
// was not able to optimize scraping multiple databases via goroutines
// while also testing with exclusively mtest.
type fakeClient struct{ mock.Mock }

func (fc *fakeClient) ListDatabaseNames(ctx context.Context, filters interface{}, opts ...*options.ListDatabasesOptions) ([]string, error) {
	args := fc.Called(ctx, filters, opts)
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

func TestListDatabaseNames(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	mont.Run("list database names", func(mt *mtest.T) {
		// mocking out a listdatabase call
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			primitive.E{
				Key: "databases",
				Value: []struct {
					Name string `bson:"name,omitempty"`
				}{
					{
						Name: "admin",
					},
				},
			}))
		driver := mt.Client
		client := &mongodbClient{
			Client: driver,
		}
		dbNames, err := client.ListDatabaseNames(context.Background(), bson.D{})
		require.NoError(t, err)
		require.Equal(t, dbNames[0], "admin")
	})

}

type commandString = string

const (
	dbStatsType      commandString = "dbStats"
	serverStatusType commandString = "serverStatus"
)

func TestRunCommands(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	loadedDbStats, err := loadDBStats()
	require.NoError(t, err)
	loadedServerStatus, err := loadServerStatus()
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
				require.Equal(t, int32(0), m["mem"].(bson.M)["mapped"])
			},
		},
	}

	for _, tc := range testCases {
		mont.Run(tc.desc, func(mt *mtest.T) {
			mt.AddMockResponses(tc.response)
			driver := mt.Client
			client := mongodbClient{
				Client: driver,
				logger: zap.NewNop(),
			}
			var result bson.M
			if tc.cmd == serverStatusType {
				result, err = client.ServerStatus(context.Background(), "test")
			} else {
				result, err = client.DBStats(context.Background(), "test")
			}
			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	buildInfo, err := loadBuildInfo()
	require.NoError(t, err)

	mont.Run("test connection", func(mt *mtest.T) {
		mt.AddMockResponses(
			// retrieving build info
			buildInfo,
		)

		driver := mt.Client
		client := mongodbClient{
			Client: driver,
			logger: zap.NewNop(),
		}

		version, err := client.GetVersion(context.TODO())
		require.NoError(t, err)
		require.Equal(t, "4.4.10", version.String())
	})
}

func TestGetVersionFailures(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	malformedBuildInfo := bson.D{
		primitive.E{Key: "ok", Value: 1},
		primitive.E{Key: "version", Value: 1},
	}

	testCases := []struct {
		desc         string
		responses    []primitive.D
		partialError string
	}{
		{
			desc:         "Unable to run buildInfo",
			responses:    []primitive.D{mtest.CreateCommandErrorResponse(mtest.CommandError{})},
			partialError: "unable to get build info",
		},
		{
			desc:         "unable to parse version",
			responses:    []primitive.D{mtest.CreateSuccessResponse(), malformedBuildInfo},
			partialError: "unable to parse mongo version from server",
		},
	}

	for _, tc := range testCases {
		mont.Run(tc.desc, func(mt *mtest.T) {
			mt.AddMockResponses(tc.responses...)
			driver := mt.Client
			client := mongodbClient{
				Client: driver,
				logger: zap.NewNop(),
			}

			_, err := client.GetVersion(context.TODO())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.partialError)
		})
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

func loadBuildInfo() (bson.D, error) {
	return loadTestFile("./testdata/buildInfo.json")
}

func loadAdminStatusAsMap() (bson.M, error) {
	return loadTestFileAsMap("./testdata/admin.json")
}

func loadTestFile(filePath string) (bson.D, error) {
	var doc bson.D
	testFile, err := ioutil.ReadFile(filePath)
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
	testFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = bson.UnmarshalExtJSON(testFile, true, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
