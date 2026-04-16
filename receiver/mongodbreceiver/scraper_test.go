// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestNewMongodbScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	require.NotEmpty(t, scraper.config.hostlist())
}

func TestGenerateInstanceID(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		// Same inputs should produce the same UUID
		id1 := generateInstanceID("localhost", 27017)
		id2 := generateInstanceID("localhost", 27017)
		require.Equal(t, id1, id2, "same inputs should produce same UUID")
		require.Equal(t, "fd638985-aee9-53f2-95d3-ce3e8483c243", id1)
	})

	t.Run("unique for different ports", func(t *testing.T) {
		id1 := generateInstanceID("localhost", 27017)
		id2 := generateInstanceID("localhost", 27018)
		require.NotEqual(t, id1, id2, "different ports should produce different UUIDs")
	})

	t.Run("unique for different hosts", func(t *testing.T) {
		id1 := generateInstanceID("host1", 27017)
		id2 := generateInstanceID("host2", 27017)
		require.NotEqual(t, id1, id2, "different hosts should produce different UUIDs")
	})

	t.Run("valid UUID v5 format", func(t *testing.T) {
		id := generateInstanceID("localhost", 27017)
		parsed, err := uuid.Parse(id)
		require.NoError(t, err, "generated ID should be a valid UUID")
		require.Equal(t, uuid.Version(5), parsed.Version(), "should be UUID v5")
	})
}

func TestDeriveOperationStatus(t *testing.T) {
	testCases := []struct {
		name     string
		op       bson.M
		expected metadata.AttributeMongodbOperationStatus
		ok       bool
	}{
		{
			name:     "active operation",
			op:       bson.M{"active": true},
			expected: metadata.AttributeMongodbOperationStatusActive,
			ok:       true,
		},
		{
			name:     "waiting for lock takes precedence",
			op:       bson.M{"active": true, "waitingForLock": true},
			expected: metadata.AttributeMongodbOperationStatusWaiting,
			ok:       true,
		},
		{
			name:     "waiting for flow control",
			op:       bson.M{"active": true, "waitingForFlowControl": true},
			expected: metadata.AttributeMongodbOperationStatusWaiting,
			ok:       true,
		},
		{
			name: "unsupported status",
			op:   bson.M{"active": false},
			ok:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := deriveOperationStatus(tc.op)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestScraperLifecycle(t *testing.T) {
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
	require.NoError(t, scraper.start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, scraper.shutdown(t.Context()))
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

			actualMetrics, err := scraper.scrape(t.Context())
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
	doc, err = client.TopStats(t.Context())
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

func TestReceiverMetricsDisabled(t *testing.T) {
	scraperCfg := createDefaultConfig().(*Config)

	// disable all metrics
	v := reflect.ValueOf(&scraperCfg.Metrics).Elem()
	for i := 0; i < v.NumField(); i++ {
		v.Field(i).FieldByName("Enabled").SetBool(false)
	}

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

	scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)
	scraper.client = fc

	scrapedMetrics, err := scraper.scrape(t.Context())
	if err != nil {
		require.NoError(t, err, "error scraping while no metrics are enabled")
	}

	require.Equal(t, 0, scrapedMetrics.MetricCount(), "no data should be scraped when all metrics are disabled")
}

func TestScrapeLogs(t *testing.T) {
	testCases := []struct {
		desc            string
		setupMockClient func(t *testing.T) *fakeClient
		expectedErr     string
		validateLogs    func(t *testing.T, logs plog.Logs)
	}{
		{
			desc: "CurrentOp returns error",
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{}, errors.New("currentOp failed"))
				return fc
			},
			expectedErr: "currentOp failed",
			validateLogs: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, 0, logs.LogRecordCount())
			},
		},
		{
			desc: "ServerStatus returns error",
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{}, errors.New("server status failed"))
				return fc
			},
			validateLogs: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, 0, logs.LogRecordCount())
			},
		},
		{
			desc: "Missing host in ServerStatus",
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"version": "4.4"}, nil)
				return fc
			},
			validateLogs: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, 0, logs.LogRecordCount())
			},
		},
		{
			desc: "Successful scrape with operations",
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{
						"ns":                "mydb.mycol",
						"op":                "query",
						"command":           bson.D{{Key: "find", Value: "mycol"}, {Key: "filter", Value: bson.D{{Key: "x", Value: 1}}}},
						"active":            true,
						"microsecs_running": int64(5000),
						"client":            "192.168.1.1:12345",
						"appName":           "testApp",
						"opid":              int32(123),
						"effectiveUsers":    bson.A{bson.M{"user": "admin"}},
					},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "mongohost:27017"}, nil)
				return fc
			},
			validateLogs: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, 1, logs.LogRecordCount())
				rl := logs.ResourceLogs().At(0)
				attrs := rl.Resource().Attributes()
				addr, ok := attrs.Get("server.address")
				require.True(t, ok)
				require.Equal(t, "mongohost", addr.Str())
				instanceID, ok := attrs.Get("service.instance.id")
				require.True(t, ok)
				require.NotEmpty(t, instanceID.Str())
			},
		},
		{
			desc: "Successful scrape with no matching operations",
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "admin.system.version", "op": "query", "command": bson.D{{Key: "hello", Value: 1}}, "active": true},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "mongohost:27017"}, nil)
				return fc
			},
			validateLogs: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, 0, logs.LogRecordCount())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraperCfg := createDefaultConfig().(*Config)
			scraperCfg.Events.DbServerQuerySample.Enabled = true
			scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)
			scraper.client = tc.setupMockClient(t)

			logs, err := scraper.scrapeLogs(t.Context())
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
			tc.validateLogs(t, logs)
		})
	}
}

func TestScrapeLogsWithSecondaries(t *testing.T) {
	testCases := []struct {
		desc                  string
		setupPrimaryClient    func(t *testing.T) *fakeClient
		setupSecondaryClients func(t *testing.T) []*fakeClient
		expectedLogCount      int
		expectedResourceCount int
		expectedErr           string
	}{
		{
			desc: "Primary and secondary both have operations",
			setupPrimaryClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.orders", "op": "query", "command": bson.D{{Key: "find", Value: "orders"}}, "active": true, "microsecs_running": int64(1000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "primary:27017"}, nil)
				return fc
			},
			setupSecondaryClients: func(t *testing.T) []*fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.products", "op": "query", "command": bson.D{{Key: "find", Value: "products"}}, "active": true, "microsecs_running": int64(2000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "secondary1:27017"}, nil)
				return []*fakeClient{fc}
			},
			expectedLogCount:      2,
			expectedResourceCount: 2,
		},
		{
			desc: "Secondary CurrentOp fails gracefully",
			setupPrimaryClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.orders", "op": "query", "command": bson.D{{Key: "find", Value: "orders"}}, "active": true, "microsecs_running": int64(1000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "primary:27017"}, nil)
				return fc
			},
			setupSecondaryClients: func(t *testing.T) []*fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{}, errors.New("secondary unreachable"))
				return []*fakeClient{fc}
			},
			expectedLogCount:      1,
			expectedResourceCount: 1,
		},
		{
			desc: "Multiple secondaries with mixed results",
			setupPrimaryClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.orders", "op": "query", "command": bson.D{{Key: "find", Value: "orders"}}, "active": true, "microsecs_running": int64(1000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "primary:27017"}, nil)
				return fc
			},
			setupSecondaryClients: func(t *testing.T) []*fakeClient {
				fc1 := &fakeClient{}
				fc1.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.products", "op": "query", "command": bson.D{{Key: "find", Value: "products"}}, "active": true, "microsecs_running": int64(500)},
				}, nil)
				fc1.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "secondary1:27017"}, nil)

				fc2 := &fakeClient{}
				fc2.On("CurrentOp", mock.Anything).Return([]bson.M{}, errors.New("secondary2 down"))

				fc3 := &fakeClient{}
				fc3.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.users", "op": "query", "command": bson.D{{Key: "find", Value: "users"}}, "active": true, "microsecs_running": int64(300)},
				}, nil)
				fc3.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "secondary3:27017"}, nil)

				return []*fakeClient{fc1, fc2, fc3}
			},
			expectedLogCount:      3,
			expectedResourceCount: 3,
		},
		{
			desc: "Primary fails returns error even with healthy secondaries",
			setupPrimaryClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{}, errors.New("primary unreachable"))
				return fc
			},
			setupSecondaryClients: func(t *testing.T) []*fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.products", "op": "query", "command": bson.D{{Key: "find", Value: "products"}}, "active": true, "microsecs_running": int64(1000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "secondary1:27017"}, nil)
				return []*fakeClient{fc}
			},
			expectedErr: "primary unreachable",
		},
		{
			desc: "No secondaries configured",
			setupPrimaryClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				fc.On("CurrentOp", mock.Anything).Return([]bson.M{
					{"ns": "mydb.orders", "op": "query", "command": bson.D{{Key: "find", Value: "orders"}}, "active": true, "microsecs_running": int64(1000)},
				}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{"host": "primary:27017"}, nil)
				return fc
			},
			setupSecondaryClients: func(t *testing.T) []*fakeClient {
				return nil
			},
			expectedLogCount:      1,
			expectedResourceCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraperCfg := createDefaultConfig().(*Config)
			scraperCfg.Events.DbServerQuerySample.Enabled = true
			scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)
			scraper.client = tc.setupPrimaryClient(t)

			secondaryFakes := tc.setupSecondaryClients(t)
			for _, fc := range secondaryFakes {
				scraper.secondaryClients = append(scraper.secondaryClients, fc)
			}

			logs, err := scraper.scrapeLogs(t.Context())
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedLogCount, logs.LogRecordCount())
			require.Equal(t, tc.expectedResourceCount, logs.ResourceLogs().Len())

			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rl := logs.ResourceLogs().At(i)
				addr, ok := rl.Resource().Attributes().Get("server.address")
				require.True(t, ok, "resource %d should have server.address", i)
				require.NotEmpty(t, addr.Str())
			}
		})
	}
}

func TestShouldIncludeOperation(t *testing.T) {
	scraper := &mongodbScraper{logger: zap.NewNop()}

	testCases := []struct {
		name     string
		op       bson.M
		expected bool
	}{
		{
			name:     "no namespace",
			op:       bson.M{"op": "query", "command": bson.D{{Key: "find", Value: 1}}},
			expected: false,
		},
		{
			name:     "empty namespace",
			op:       bson.M{"ns": "", "op": "query", "command": bson.D{{Key: "find", Value: 1}}},
			expected: false,
		},
		{
			name:     "admin database",
			op:       bson.M{"ns": "admin.system.version", "op": "query", "command": bson.D{{Key: "find", Value: 1}}},
			expected: false,
		},
		{
			name:     "no command",
			op:       bson.M{"ns": "mydb.mycol"},
			expected: false,
		},
		{
			name:     "empty command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{}},
			expected: false,
		},
		{
			name:     "hello command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{{Key: "hello", Value: 1}}},
			expected: false,
		},
		{
			name:     "ping command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{{Key: "ping", Value: 1}}},
			expected: false,
		},
		{
			name:     "isMaster command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{{Key: "isMaster", Value: 1}}},
			expected: false,
		},
		{
			name:     "valid find command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{{Key: "find", Value: "mycol"}}},
			expected: true,
		},
		{
			name:     "valid insert command",
			op:       bson.M{"ns": "mydb.mycol", "command": bson.D{{Key: "insert", Value: "mycol"}}},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, scraper.shouldIncludeOperation(tc.op))
		})
	}
}

func TestGetDBFromNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		expected  string
	}{
		{"mydb.mycol", "mydb"},
		{"admin.system.version", "admin"},
		{"nodot", ""},
		{"", ""},
		{"db.col.subcol", "db"},
	}
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			require.Equal(t, tt.expected, getDBFromNamespace(tt.namespace))
		})
	}
}

func TestGetCollectionFromNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		expected  string
	}{
		{"mydb.mycol", "mycol"},
		{"admin.system.version", "system.version"},
		{"nodot", ""},
		{"", ""},
		{"db.col.subcol", "col.subcol"},
	}
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			require.Equal(t, tt.expected, getCollectionFromNamespace(tt.namespace))
		})
	}
}

func TestExtractEffectiveUserName(t *testing.T) {
	testCases := []struct {
		name     string
		op       bson.M
		expected string
	}{
		{
			name:     "no effectiveUsers key",
			op:       bson.M{},
			expected: "",
		},
		{
			name:     "empty effectiveUsers",
			op:       bson.M{"effectiveUsers": bson.A{}},
			expected: "",
		},
		{
			name:     "effectiveUsers with bson.M",
			op:       bson.M{"effectiveUsers": bson.A{bson.M{"user": "admin", "db": "test"}}},
			expected: "admin",
		},
		{
			name:     "effectiveUsers with bson.D",
			op:       bson.M{"effectiveUsers": bson.A{bson.D{{Key: "user", Value: "dbowner"}, {Key: "db", Value: "mydb"}}}},
			expected: "dbowner",
		},
		{
			name:     "effectiveUsers with map[string]any",
			op:       bson.M{"effectiveUsers": bson.A{map[string]any{"user": "mapuser"}}},
			expected: "mapuser",
		},
		{
			name:     "effectiveUsers with bson.M missing user key",
			op:       bson.M{"effectiveUsers": bson.A{bson.M{"db": "test"}}},
			expected: "",
		},
		{
			name:     "effectiveUsers with bson.D missing user key",
			op:       bson.M{"effectiveUsers": bson.A{bson.D{{Key: "db", Value: "mydb"}}}},
			expected: "",
		},
		{
			name:     "effectiveUsers with unsupported type",
			op:       bson.M{"effectiveUsers": bson.A{"stringvalue"}},
			expected: "",
		},
		{
			name:     "effectiveUsers wrong type",
			op:       bson.M{"effectiveUsers": "notanarray"},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, extractEffectiveUserName(tc.op))
		})
	}
}

func TestExtractOperationID(t *testing.T) {
	testCases := []struct {
		name     string
		op       bson.M
		expected string
	}{
		{
			name:     "integer opid",
			op:       bson.M{"opid": int32(12345)},
			expected: "12345",
		},
		{
			name:     "string opid",
			op:       bson.M{"opid": "shard1:12345"},
			expected: "shard1:12345",
		},
		{
			name:     "missing opid",
			op:       bson.M{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, extractOperationID(tc.op))
		})
	}
}

func TestProcessCurrentOp(t *testing.T) {
	scraperCfg := createDefaultConfig().(*Config)
	scraperCfg.Events.DbServerQuerySample.Enabled = true
	scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)

	operations := []bson.M{
		{
			"ns":                "mydb.orders",
			"op":                "query",
			"command":           bson.D{{Key: "find", Value: "orders"}, {Key: "filter", Value: bson.D{{Key: "status", Value: "active"}}}},
			"active":            true,
			"microsecs_running": int64(2500000),
			"client":            "10.0.0.1:54321",
			"appName":           "orderService",
			"opid":              int32(999),
			"effectiveUsers":    bson.A{bson.M{"user": "appuser"}},
		},
		{
			"ns":                "mydb.products",
			"op":                "update",
			"command":           bson.D{{Key: "update", Value: "products"}},
			"active":            true,
			"waitingForLock":    true,
			"microsecs_running": int64(100000),
			"client":            "10.0.0.2:54322",
		},
		// Should be skipped: admin namespace
		{
			"ns":      "admin.system.version",
			"op":      "query",
			"command": bson.D{{Key: "find", Value: 1}},
			"active":  true,
		},
		// Should be skipped: inactive operation
		{
			"ns":      "mydb.orders",
			"op":      "query",
			"command": bson.D{{Key: "find", Value: "orders"}},
			"active":  false,
		},
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	scraper.processCurrentOp(t.Context(), operations, now)

	logs := scraper.lb.Emit()
	require.Equal(t, 2, logs.LogRecordCount(), "only 2 of 4 operations should produce log records")
}

func TestDependentMetricsWhenDisabled(t *testing.T) {
	tests := []struct {
		name              string
		subject           func(*metadata.MetricsConfig)
		dependent         func(*metadata.MetricsConfig)
		expectedMetricGen func(t *testing.T) pmetric.Metrics
	}{
		{
			name: "mongodb.commands.rate metric should work when mongodb.operation.count disabled",
			subject: func(mc *metadata.MetricsConfig) {
				mc.MongodbOperationCount.Enabled = false
			},
			dependent: func(mc *metadata.MetricsConfig) {
				mc.MongodbCommandsRate.Enabled = true
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "mongodb-commands-rate-count-dependency.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
		},
		{
			name: "mongodb.repl_commands_per_sec metric should work when mongodb.operation.repl.count disabled",
			subject: func(mc *metadata.MetricsConfig) {
				mc.MongodbOperationReplCount.Enabled = false
			},
			dependent: func(mc *metadata.MetricsConfig) {
				mc.MongodbReplCommandsPerSec.Enabled = true
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "mongodb-repl-commands-per-sec-count-dependency.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraperCfg := createDefaultConfig().(*Config)

			tt.subject(&scraperCfg.Metrics)
			tt.dependent(&scraperCfg.Metrics)

			// successful scrape config
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

			scraper := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), scraperCfg)
			scraper.client = fc

			_, err = scraper.scrape(t.Context())
			if err != nil {
				require.NoError(t, err, "error scraping metrics")
			}

			// wait a few seconds, then scrape again so metrics that rely on previous values can be calculated
			time.Sleep(2 * time.Second)
			scrapedMetrics, err := scraper.scrape(t.Context())
			if err != nil {
				require.NoError(t, err, "error scraping metrics")
			}

			expectedMetrics := tt.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, scrapedMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}
