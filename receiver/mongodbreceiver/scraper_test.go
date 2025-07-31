// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestNewMongodbScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	require.NotEmpty(t, scraper.config.hostlist())
}

func TestScraperLifecycle(t *testing.T) {
	now := time.Now()
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	/*
		NOTE:
		setting direct connection to true because originally, the scraper tests only ONE mongodb instance.
		added in routing logic to detect multiple mongodb instances which takes longer than 2 milliseconds.
		since this test is testing for lifecycle (start and shutting down ONE instance).
	*/
	cfg.DirectConnection = true

	scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, scraper.shutdown(context.Background()))

	require.Less(t, time.Since(now), 200*time.Millisecond, "component start and stop should be very fast")
}

var (
	errAllPartialMetrics = errors.New(
		strings.Join(
			[]string{
				"failed to collect metric mongodb.cache.operations with attribute(s) miss, hit: could not find key for metric",
				"failed to collect metric mongodb.cursor.count: could not find key for metric",
				"failed to collect metric mongodb.cursor.timeout.count: could not find key for metric",
				"failed to collect metric mongodb.global_lock.time: could not find key for metric",
				"failed to collect metric bytesIn: could not find key for metric",
				"failed to collect metric bytesOut: could not find key for metric",
				"failed to collect metric numRequests: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) delete: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) getmore: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) insert: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) query: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) update: could not find key for metric",
				"failed to collect metric mongodb.session.count: could not find key for metric",
				"failed to collect metric mongodb.operation.time: could not find key for metric",
				"failed to collect metric mongodb.collection.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.data.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.extent.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.index.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.index.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.object.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.storage.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) available, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) current, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) active, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) inserted, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) updated, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) deleted, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.memory.usage with attribute(s) resident, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.memory.usage with attribute(s) virtual, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.index.access.count with attribute(s) fakedatabase, orders: could not find key for index access metric",
				"failed to collect metric mongodb.index.access.count with attribute(s) fakedatabase, products: could not find key for index access metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) read: could not find key for metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) write: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) delete: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) getmore: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) insert: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) query: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) update: could not find key for metric",
				"failed to collect metric mongodb.health: could not find key for metric",
				"failed to collect metric mongodb.uptime: could not find key for metric",
				"failed to collect metric mongodb.active.reads: could not find key for metric",
				"failed to collect metric mongodb.active.writes: could not find key for metric",
				"failed to collect metric mongodb.flushes.rate: could not find key for metric",
				"failed to collect metric mongodb.page_faults: could not find key for metric",
				"failed to collect metric mongodb.wtcache.bytes.read: could not find key for metric",
			}, "; "))
	errAllClientFailedFetch = errors.New(
		strings.Join(
			[]string{
				"failed to fetch top stats metrics: some top stats error",
				"failed to fetch database stats metrics: some database stats error",
				"failed to fetch server status metrics: some server status error",
				"failed to fetch index stats metrics: some index stats error",
				"failed to fetch index stats metrics: some index stats error",
			}, "; "))

	errCollectionNames = errors.New(
		strings.Join(
			[]string{
				"failed to fetch top stats metrics: some top stats error",
				"failed to fetch database stats metrics: some database stats error",
				"failed to fetch server status metrics: some server status error",
				"failed to fetch collection names: some collection names error",
			}, "; "))
)

func TestScraperScrape(t *testing.T) {
	testCases := []struct {
		desc              string
		partialErr        bool
		setupMockClient   func(t *testing.T) *fakeClient
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
	}{
		{
			desc:       "Nil client",
			partialErr: false,
			setupMockClient: func(*testing.T) *fakeClient {
				return nil
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("no client was initialized before calling scrape"),
		},
		{
			desc:       "Failed to fetch database names",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{}, errors.New("some database names error"))
				return fc
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("failed to fetch database names: some database names error"),
		},
		{
			desc:       "Failed to fetch collection names",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				adminStatus, err := loadAdminStatusAsMap()
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some server status error"))
				fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some database stats error"))
				fc.On("TopStats", mock.Anything).Return(bson.M{}, errors.New("some top stats error"))
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{}, errors.New("some collection names error"))
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "partial_scrape.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errCollectionNames,
		},
		{
			desc:       "Failed to scrape client stats",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				adminStatus, err := loadAdminStatusAsMap()
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some server status error"))
				fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some database stats error"))
				fc.On("TopStats", mock.Anything).Return(bson.M{}, errors.New("some top stats error"))
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return([]bson.M{}, errors.New("some index stats error"))
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return([]bson.M{}, errors.New("some index stats error"))
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "partial_scrape.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errAllClientFailedFetch,
		},
		{
			desc:       "Failed to scrape with partial errors on metrics",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				wiredTigerStorage, err := loadOnlyStorageEngineAsMap()
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				indexStats, err := loadIndexStatsAsMap("error")
				require.NoError(t, err)
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(wiredTigerStorage, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, nil)
				fc.On("TopStats", mock.Anything).Return(bson.M{}, nil)
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return(indexStats, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return(indexStats, nil)
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "db_count_only.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errAllPartialMetrics,
		},
		{
			desc:       "Successful scrape",
			partialErr: false,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				adminStatus, err := loadAdminStatusAsMap()
				require.NoError(t, err)
				ss, err := loadServerStatusAsMap()
				require.NoError(t, err)
				dbStats, err := loadDBStatsAsMap()
				require.NoError(t, err)
				topStats, err := loadTopAsMap()
				require.NoError(t, err)
				productsIndexStats, err := loadIndexStatsAsMap("products")
				require.NoError(t, err)
				ordersIndexStats, err := loadIndexStatsAsMap("orders")
				require.NoError(t, err)
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(ss, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(dbStats, nil)
				fc.On("TopStats", mock.Anything).Return(topStats, nil)
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return(productsIndexStats, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return(ordersIndexStats, nil)
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "expected.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraperCfg := createDefaultConfig().(*Config)
			// Enable any metrics set to `false` by default
			scraperCfg.Metrics.MongodbOperationLatencyTime.Enabled = true
			scraperCfg.Metrics.MongodbOperationReplCount.Enabled = true
			scraperCfg.Metrics.MongodbUptime.Enabled = true
			scraperCfg.Metrics.MongodbHealth.Enabled = true

			scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)

			mc := tc.setupMockClient(t)
			if mc != nil {
				scraper.client = mc
			}

			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				if strings.Contains(err.Error(), ";") {
					// metrics with attributes use a map and errors can be returned in random order so sorting is required.
					// The first error message would not have a leading whitespace and hence split on "; "
					actualErrs := strings.Split(err.Error(), "; ")
					sort.Strings(actualErrs)
					// The first error message would not have a leading whitespace and hence split on "; "
					expectedErrs := strings.Split(tc.expectedErr.Error(), "; ")
					sort.Strings(expectedErrs)
					require.Equal(t, expectedErrs, actualErrs)
				} else {
					require.EqualError(t, err, tc.expectedErr.Error())
				}
			}

			if mc != nil {
				mc.AssertExpectations(t)
			}

			if tc.partialErr {
				require.True(t, scrapererror.IsPartialScrapeError(err))
			} else {
				require.False(t, scrapererror.IsPartialScrapeError(err))
			}
			expectedMetrics := tc.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}

func TestTopMetricsAggregation(t *testing.T) {
	mt := drivertest.NewMockDeployment()
	opts := options.Client()
	//nolint:staticcheck // Using deprecated Deployment field for testing purposes
	opts.Deployment = mt
	c, err := mongo.Connect(opts)
	require.NoError(t, err)

	loadedTop, err := loadTop()
	require.NoError(t, err)

	mt.AddResponses(loadedTop)
	client := mongodbClient{
		Client: c,
		logger: zap.NewNop(),
	}
	var doc bson.M
	doc, err = client.TopStats(context.Background())
	require.NoError(t, err)

	collectionPathNames, err := digForCollectionPathNames(doc)
	require.NoError(t, err)
	require.ElementsMatch(t, collectionPathNames,
		[]string{
			"config.transactions",
			"test.admin",
			"test.orders",
			"admin.system.roles",
			"local.system.replset",
			"test.products",
			"admin.system.users",
			"admin.system.version",
			"config.system.sessions",
			"local.oplog.rs",
			"local.startup_log",
		})

	actualOperationTimeValues, err := aggregateOperationTimeValues(doc, collectionPathNames, operationsMap)
	require.NoError(t, err)

	// values are taken from testdata/top.json
	expectedInsertValues := 0 + 0 + 0 + 0 + 0 + 11302 + 0 + 1163 + 0 + 0 + 0
	expectedQueryValues := 0 + 0 + 6072 + 0 + 0 + 0 + 44 + 0 + 0 + 0 + 2791
	expectedUpdateValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 155 + 9962 + 0
	expectedRemoveValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 3750 + 0
	expectedGetmoreValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0
	expectedCommandValues := 540 + 397 + 4009 + 0 + 0 + 23285 + 0 + 10993 + 0 + 10116 + 0
	require.EqualValues(t, expectedInsertValues, actualOperationTimeValues["insert"])
	require.EqualValues(t, expectedQueryValues, actualOperationTimeValues["queries"])
	require.EqualValues(t, expectedUpdateValues, actualOperationTimeValues["update"])
	require.EqualValues(t, expectedRemoveValues, actualOperationTimeValues["remove"])
	require.EqualValues(t, expectedGetmoreValues, actualOperationTimeValues["getmore"])
	require.EqualValues(t, expectedCommandValues, actualOperationTimeValues["commands"])
}

func TestServerAddressAndPort(t *testing.T) {
	tests := []struct {
		name            string
		serverStatus    bson.M
		expectedAddress string
		expectedPort    int64
		expectedErr     error
	}{
		{
			name: "address_only",
			serverStatus: bson.M{
				"host": "localhost",
			},
			expectedAddress: "localhost",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name: "address_and_port",
			serverStatus: bson.M{
				"host": "localhost:27018",
			},
			expectedAddress: "localhost",
			expectedPort:    27018,
		},
		{
			name:         "missing_host",
			serverStatus: bson.M{},
			expectedErr:  errors.New("host field not found in server status"),
		},
		{
			name: "invalid_port",
			serverStatus: bson.M{
				"host": "localhost:invalid",
			},
			expectedErr: errors.New("failed to parse port: strconv.ParseInt: parsing \"invalid\": invalid syntax"),
		},
		{
			name: "invalid_host_format",
			serverStatus: bson.M{
				"host": "localhost:27018:extra",
			},
			expectedErr: errors.New("unexpected host format: localhost:27018:extra"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, port, err := serverAddressAndPort(tt.serverStatus)
			if tt.expectedErr != nil {
				require.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAddress, address)
				require.Equal(t, tt.expectedPort, port)
			}
		})
	}
}

func TestShouldIncludeOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation bson.M
		expected  bool
	}{
		{
			name: "valid query operation",
			operation: bson.M{
				"ns":      "test.collection",
				"op":      "query",
				"command": bson.D{{Key: "find", Value: "collection"}},
			},
			expected: true,
		},
		{
			name: "missing namespace",
			operation: bson.M{
				"op":      "query",
				"command": bson.D{{Key: "find", Value: "collection"}},
			},
			expected: false,
		},
		{
			name: "admin database",
			operation: bson.M{
				"ns":      "admin.collection",
				"op":      "query",
				"command": bson.D{{Key: "find", Value: "collection"}},
			},
			expected: false,
		},
		{
			name: "hello command",
			operation: bson.M{
				"ns":      "test.collection",
				"op":      "query",
				"command": bson.D{{Key: "hello", Value: 1}},
			},
			expected: false,
		},
	}

	s := &mongodbScraper{logger: zap.NewNop()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, s.shouldIncludeOperation(tt.operation))
		})
	}
}

func TestShouldExplainOperation(t *testing.T) {
	tests := []struct {
		name     string
		opType   string
		command  bson.D
		expected bool
	}{
		{
			name:     "query operation",
			opType:   "query",
			command:  bson.D{{Key: "find", Value: "collection"}},
			expected: true,
		},
		{
			name:     "insert operation",
			opType:   "insert",
			command:  bson.D{{Key: "insert", Value: "collection"}},
			expected: false,
		},
		{
			name:     "unexplainable command",
			opType:   "query",
			command:  bson.D{{Key: "getMore", Value: 12345}},
			expected: false,
		},
		{
			name:     "unexplainable pipeline stage",
			opType:   "query",
			command:  bson.D{{Key: "aggregate", Value: "collection"}, {Key: "pipeline", Value: bson.A{bson.M{"$collStats": bson.M{}}}}},
			expected: false,
		},
	}

	s := &mongodbScraper{logger: zap.NewNop()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, s.shouldExplainOperation(tt.opType, tt.command))
		})
	}
}

func TestGetExplainPlan(t *testing.T) {
	mockClient := &fakeClient{}
	s := &mongodbScraper{
		client: mockClient,
		logger: zap.NewNop(),
	}

	ctx := context.Background()
	command := bson.D{{Key: "find", Value: "collection"}}

	// Test successful explain plan
	mockClient.On("RunCommand", ctx, "admin", bson.M{"explain": command}).Return(bson.M{
		"queryPlanner": bson.M{
			"winningPlan": bson.M{
				"stage": "COLLSCAN",
			},
		},
	}, nil).Once()

	plan, err := s.getExplainPlan(ctx, command)
	require.NoError(t, err)
	require.Contains(t, plan, "queryPlanner")

	// Test command preparation
	commandWithExtra := bson.D{
		{Key: "find", Value: "collection"},
		{Key: "$db", Value: "testdb"},
		{Key: "comment", Value: "test"},
	}
	mockClient.On("RunCommand", ctx, "admin", bson.M{"explain": bson.D{
		{Key: "find", Value: "collection"},
		{Key: "$db", Value: "testdb"},
	}}).Return(bson.M{}, nil)
	_, err = s.getExplainPlan(ctx, commandWithExtra)
	require.NoError(t, err)

	// Test error case
	mockClient.On("RunCommand", ctx, "admin", mock.Anything).Return(bson.M{}, errors.New("explain failed")).Once()
	_, err = s.getExplainPlan(ctx, command)
	require.Error(t, err)
}

func TestProcessCurrentOp(t *testing.T) {
	mockClient := &fakeClient{}
	lb := metadata.NewLogsBuilder(metadata.DefaultLogsBuilderConfig(), receivertest.NewNopSettings(metadata.Type))
	s := &mongodbScraper{
		client:     mockClient,
		logger:     zap.NewNop(),
		lb:         lb,
		obfuscator: newObfuscator(),
	}

	ctx := context.Background()
	now := pcommon.NewTimestampFromTime(time.Now())

	tests := []struct {
		name         string
		operations   []bson.M
		mockExplain  bool
		expectedLogs int
		collection   []string
	}{
		{
			name: "query operation",
			operations: []bson.M{
				{
					"ns":                "test.users",
					"op":                "query",
					"command":           bson.D{{Key: "find", Value: "users"}, {Key: "filter", Value: bson.M{"name": "test"}}},
					"microsecs_running": int64(100),
					"appName":           "testapp",
					"client":            "127.0.0.1:27017",
				},
			},
			mockExplain:  true,
			expectedLogs: 1,
			collection:   []string{"users"},
		},
		{
			name: "insert operation",
			operations: []bson.M{
				{
					"ns":                "test.users",
					"op":                "insert",
					"command":           bson.D{{Key: "insert", Value: "users"}, {Key: "documents", Value: bson.A{bson.M{"name": "test"}}}},
					"microsecs_running": int64(50),
				},
			},
			expectedLogs: 1,
			collection:   []string{"users"},
		},
		{
			name: "admin database operation",
			operations: []bson.M{
				{
					"ns":      "admin.users",
					"op":      "query",
					"command": bson.D{{Key: "find", Value: "users"}},
				},
			},
			expectedLogs: 0,
			collection:   []string{"users"},
		},
		{
			name: "multiple operations",
			operations: []bson.M{
				{
					"ns":                "test.users",
					"op":                "query",
					"command":           bson.D{{Key: "find", Value: "users"}},
					"microsecs_running": int64(100),
				},
				{
					"ns":                "test.products",
					"op":                "query",
					"command":           bson.D{{Key: "find", Value: "products"}},
					"microsecs_running": int64(200),
				},
			},
			mockExplain:  true,
			expectedLogs: 2,
			collection:   []string{"users", "products"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockExplain {
				mockClient.On("RunCommand", ctx, "admin", mock.Anything).Return(bson.M{
					"queryPlanner": bson.M{
						"winningPlan": bson.M{
							"stage": "COLLSCAN",
						},
					},
				}, nil)
			}

			s.processCurrentOp(ctx, tt.operations, now)
			logs := s.lb.Emit()
			require.Equal(t, tt.expectedLogs, logs.LogRecordCount())

			// Verify log attributes for first operation if expected
			for i := 0; i < tt.expectedLogs; i++ {
				lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(i)
				require.Equal(t, now, lr.Timestamp())
				dbName, ok := lr.Attributes().Get("db.collection.name")
				require.True(t, ok)
				require.Equal(t, tt.collection[i], dbName.Str())
				require.Contains(t, lr.Attributes().AsRaw(), "mongodb.query.signature")
			}
		})
	}
}

func TestScrapeLogs(t *testing.T) {
	mockClient := &fakeClient{}
	lb := metadata.NewLogsBuilder(metadata.DefaultLogsBuilderConfig(), receivertest.NewNopSettings(metadata.Type))
	s := &mongodbScraper{
		client:     mockClient,
		logger:     zap.NewNop(),
		lb:         lb,
		obfuscator: newObfuscator(),
	}

	ctx := context.Background()

	// Test successful case
	mockClient.On("CurrentOp", ctx).Return([]bson.M{
		{
			"ns":                "test.users",
			"op":                "query",
			"command":           bson.D{{Key: "find", Value: "users"}},
			"microsecs_running": int64(100),
			"appName":           "testapp",
			"client":            "127.0.0.1:27017",
		},
	}, nil).Once()
	mockClient.On("RunCommand", ctx, "admin", bson.M{"explain": bson.D{bson.E{Key: "find", Value: "users"}}}).Return(bson.M{}, nil).Once()
	mockClient.On("ServerStatus", ctx, "admin").Return(bson.M{"host": "localhost:27017"}, nil).Once()

	logs, err := s.scrapeLogs(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, logs.LogRecordCount())
	mockClient.AssertExpectations(t)
	// golden.WriteLogs(t, filepath.Join("testdata", "query_sample.yaml"), logs)
	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "query_sample.yaml"))
	assert.NoError(t, err)
	assert.NoError(t, plogtest.CompareLogs(expectedLogs, logs,
		plogtest.IgnoreTimestamp()))

	// Test error on CurrentOp
	mockClient.On("CurrentOp", ctx).Return([]bson.M{}, errors.New("current op failed")).Once()
	_, err = s.scrapeLogs(ctx)
	require.Error(t, err)
	mockClient.AssertExpectations(t)

	// Test error on ServerStatus
	mockClient.On("CurrentOp", ctx).Return([]bson.M{}, nil).Once()
	mockClient.On("ServerStatus", ctx, "admin").Return(bson.M{}, errors.New("server status failed")).Once()
	logs, err = s.scrapeLogs(ctx)
	require.NoError(t, err) // Should not return error, just log it
	require.Equal(t, 0, logs.LogRecordCount())
	mockClient.AssertExpectations(t)
}
