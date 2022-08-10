// Copyright The OpenTelemetry Authors
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
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestNewMongodbScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
	require.NotEmpty(t, scraper.config.hostlist())
}

func TestScraperLifecycle(t *testing.T) {
	now := time.Now()
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, scraper.shutdown(context.Background()))

	require.Less(t, time.Since(now), 100*time.Millisecond, "component start and stop should be very fast")
}

func TestScrape(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

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
	fc := &fakeClient{}
	fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
	fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(ss, nil)
	fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
	fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(dbStats, nil)
	fc.On("TopStats", mock.Anything).Return(topStats, nil)
	fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
	fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return(productsIndexStats, nil)
	fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return(ordersIndexStats, nil)

	scraper := mongodbScraper{
		client:       fc,
		config:       cfg,
		mb:           metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings(), componenttest.NewNopReceiverCreateSettings().BuildInfo),
		logger:       zap.NewNop(),
		mongoVersion: mongo40,
	}

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))
}

func TestScrapeNoClient(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := &mongodbScraper{
		logger: zap.NewNop(),
		config: cfg,
	}

	m, err := scraper.scrape(context.Background())
	require.Zero(t, m.MetricCount())
	require.Error(t, err)
}

func TestGlobalLockTimeOldFormat(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Metrics = metadata.DefaultMetricsSettings()
	scraper := newMongodbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
	mong26, err := version.NewVersion("2.6")
	require.NoError(t, err)
	scraper.mongoVersion = mong26
	doc := primitive.M{
		"locks": primitive.M{
			".": primitive.M{
				"timeLockedMicros": primitive.M{
					"R": 122169,
					"W": 132712,
				},
				"timeAcquiringMicros": primitive.M{
					"R": 116749,
					"W": 14340,
				},
			},
		},
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	scraper.recordGlobalLockTime(now, doc, &scrapererror.ScrapeErrors{})
	expectedValue := (int64(116749+14340) / 1000)

	metrics := scraper.mb.Emit().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	collectedValue := metrics.At(0).Sum().DataPoints().At(0).IntVal()
	require.Equal(t, expectedValue, collectedValue)
}

func TestTopMetricsAggregation(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	loadedTop, err := loadTop()
	require.NoError(t, err)

	mont.Run("test top stats are aggregated correctly", func(mt *mtest.T) {
		mt.AddMockResponses(loadedTop)
		driver := mt.Client
		client := mongodbClient{
			Client: driver,
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
	})
}
