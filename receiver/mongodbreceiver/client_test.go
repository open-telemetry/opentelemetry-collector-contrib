package mongodbreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

var _ driver = (*fakeDriver)(nil)

// fakeDriver is an underlying mock of the mongo.Client used explicitly for testing
// some functionality wants to be tested outside of the scope of mtest
// such as some unclean disconnections and it is easier to mock
// the function behavior rather than specifying the raw bson results
type fakeDriver struct {
	mock.Mock
}

func (c *fakeDriver) Disconnect(ctx context.Context) error {
	args := c.Called(ctx)
	return args.Error(0)
}

func (c *fakeDriver) RunCommand(ctx context.Context, database string, command bson.M) (bson.M, error) {
	args := c.Called(ctx, database, command)
	return args.Get(0).(bson.M), args.Error(1)
}

func (c *fakeDriver) DBStats(ctx context.Context, database string) (bson.M, error) {
	args := c.Called(ctx, database)
	return args.Get(0).(bson.M), args.Error(1)
}

func (c *fakeDriver) ServerStatus(ctx context.Context, database string) (bson.M, error) {
	args := c.Called(ctx, database)
	return args.Get(0).(bson.M), args.Error(1)
}

func (c *fakeDriver) ListDatabaseNames(ctx context.Context, filter interface{}, options ...*options.ListDatabasesOptions) ([]string, error) {
	args := c.Called(ctx, filter, options)
	return args.Get(0).([]string), args.Error(1)
}

func (c *fakeDriver) Connect(ctx context.Context) error {
	args := c.Called(ctx)
	return args.Error(0)
}

func (c *fakeDriver) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	args := c.Called(ctx, rp)
	return args.Error(0)
}

func (c *fakeDriver) Database(name string, opts ...*options.DatabaseOptions) *mongo.Database {
	args := c.Called(name, opts)
	return args.Get(0).(*mongo.Database)
}

func (c *fakeDriver) TestConnection(ctx context.Context) (*mongoTestConnectionResponse, error) {
	args := c.Called(ctx)
	return args.Get(0).(*mongoTestConnectionResponse), args.Error(1)
}

func TestValidClient(t *testing.T) {
	client, err := NewClient(&Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		Hosts: []confignet.NetAddr{
			{
				Endpoint: "localhost:27017",
			},
		},
		Username: "username",
		Password: "password",
		Timeout:  1 * time.Second,
	}, zap.NewNop())

	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestBadTLSClient(t *testing.T) {
	client, err := NewClient(&Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		Hosts: []confignet.NetAddr{
			{
				Endpoint: "localhost:27017",
			},
		},
		TLSClientSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   "/dev/temporal.txt",
				CertFile: "/dev/test.txt",
			},
		},
		Username: "username",
		Password: "password",
		Timeout:  1 * time.Second,
	}, zap.NewNop())

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid tls configuration")
	require.Nil(t, client)
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
			driver: driver,
			logger: zap.NewNop(),
		}
		dbNames, err := client.ListDatabaseNames(context.TODO(), bson.D{})
		require.NoError(t, err)
		require.Equal(t, dbNames[0], "admin")
	})

}

func TestBadClientNonTCPAddr(t *testing.T) {
	cl, err := NewClient(&Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		Hosts: []confignet.NetAddr{
			{
				Endpoint: "/dev/null",
			},
		},
		Timeout: 1 * time.Second,
	}, zap.NewNop())

	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = cl.Connect(ctx)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "unable to instantiate mongo client")
}

func TestInitClientBadEndpoint(t *testing.T) {
	client := mongodbClient{
		username: "admin",
		password: "password",
		hosts:    []string{"x:localhost:27017:an_invalid_tcp_addr"},
		logger:   zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := client.ensureClient(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error creating")
}

func TestInitClientReplicaSet(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	mont.Run("ping failure", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateCommandErrorResponse(mtest.CommandError{}),
			mtest.CreateSuccessResponse(),
			mtest.CreateSuccessResponse(),
		)
		driver := mt.Client
		client := mongodbClient{
			driver: driver,
			hosts:  []string{"localhost:27017"},
			logger: zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := client.ensureClient(ctx)
		require.NoError(t, err)
	})

}

func TestDisconnectSuccess(t *testing.T) {
	driver := &fakeDriver{}
	driver.On("Disconnect", mock.Anything).Return(nil)
	driver.On("Ping", mock.Anything, mock.Anything).Return(nil)

	client := mongodbClient{
		driver: driver,
		logger: zap.NewNop(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.Disconnect(ctx)
	require.NoError(t, err)
}

func TestDisconnectFailure(t *testing.T) {
	fakeMongoClient := &fakeDriver{}
	fakeMongoClient.On("Disconnect", mock.Anything).Return(fmt.Errorf("connection terminated by peer"))
	fakeMongoClient.On("Ping", mock.Anything, mock.Anything).Return(nil)

	client := mongodbClient{
		driver: fakeMongoClient,
		logger: zap.NewNop(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := client.Disconnect(ctx)
	require.Error(t, err)
}

func TestDisconnectNoClient(t *testing.T) {
	client := mongodbClient{
		logger: zap.NewNop(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := client.Disconnect(ctx)
	require.NoError(t, err)
}

type commandString = string

const (
	dbStatsType      commandString = "dbStats"
	serverStatusType commandString = "serverStatus"
)

func TestSuccessfulRunCommands(t *testing.T) {
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
				require.Equal(t, int32(1), m["connections"].(bson.M)["active"])
			},
		},
	}

	for _, tc := range testCases {
		mont.Run(tc.desc, func(mt *mtest.T) {
			mt.AddMockResponses(tc.response)
			driver := mt.Client
			client := mongodbClient{
				driver: driver,
				logger: zap.NewNop(),
			}
			var result bson.M
			if tc.cmd == serverStatusType {
				result, err = client.ServerStatus(context.TODO(), "test")
			} else {
				result, err = client.DBStats(context.TODO(), "test")
			}
			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}

func TestTestConnectionSuccess(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	buildInfo, err := loadBuildInfo()
	require.NoError(t, err)

	mont.Run("test connection", func(mt *mtest.T) {
		mt.AddMockResponses(
			// Connection Success
			mtest.CreateSuccessResponse(),
			// retrieving build info
			buildInfo,
		)

		driver := mt.Client
		client := mongodbClient{
			driver: driver,
			logger: zap.NewNop(),
		}

		res, err := client.TestConnection(context.TODO())
		require.NoError(t, err)
		require.Equal(t, "4.4.10", res.Version)
	})
}

func TestTestConnectionFailures(t *testing.T) {
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
			desc:         "Failure to connect",
			responses:    []primitive.D{},
			partialError: "unable to connect",
		},
		{
			desc:         "Unable to run buildInfo",
			responses:    []primitive.D{mtest.CreateSuccessResponse()},
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
				driver: driver,
				logger: zap.NewNop(),
			}

			_, err := client.TestConnection(context.TODO())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.partialError)
		})
	}

}

func loadDBStats() (bson.D, error) {
	return loadTestFile("./testdata/dbstats.json")
}

func loadServerStatus() (bson.D, error) {
	return loadTestFile("./testdata/serverStatus.json")
}

func loadBuildInfo() (bson.D, error) {
	return loadTestFile("./testdata/buildInfo.json")
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
