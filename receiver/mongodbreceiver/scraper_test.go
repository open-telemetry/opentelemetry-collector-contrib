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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	mongo40, err := version.NewVersion("4.0")
	require.NoError(t, err)

	fakeDatabaseName := "fakedatabase"
	fc := &fakeClient{}
	fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
	fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(ss, nil)
	fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
	fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(dbStats, nil)

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
	scraper.recordGlobalLockTime(now, doc, scrapererror.ScrapeErrors{})
	expectedValue := (int64(116749+14340) / 1000)

	metrics := scraper.mb.Emit().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	collectedValue := metrics.At(0).Sum().DataPoints().At(0).IntVal()
	require.Equal(t, expectedValue, collectedValue)
}
