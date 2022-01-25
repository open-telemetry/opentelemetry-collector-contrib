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
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestNewMongodbScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(zap.NewNop(), cfg)
	require.NotEmpty(t, scraper.config.hostlist())
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
		mb:           metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
		logger:       zap.NewNop(),
		mongoVersion: mongo40,
	}

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	requireMetricsEqual(t, actualMetrics, expectedMetrics)
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
	scraper := newMongodbScraper(zap.NewNop(), cfg)
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

	now := pdata.NewTimestampFromTime(time.Now())
	scraper.recordGlobalLockTime(now, doc, scrapererror.ScrapeErrors{})
	expectedValue := (int64(116749+14340) / 1000)

	metrics := pdata.NewMetricSlice()
	scraper.mb.EmitAdmin(metrics)
	collectedValue := metrics.At(0).Sum().DataPoints().At(0).IntVal()
	require.Equal(t, expectedValue, collectedValue)
}

func requireMetricsEqual(t *testing.T, m1, m2 pdata.Metrics) {
	rms1 := m1.ResourceMetrics()
	rms2 := m2.ResourceMetrics()

	if rms1.Len() != rms2.Len() {
		require.Fail(t, "First metric had %d resource metrics, second had %d", rms1.Len(), rms2.Len())
	}

	for i := 0; i < rms1.Len(); i++ {
		rm1 := rms1.At(i)
		rm2 := rms2.At(i)

		require.Equal(t, rm1.Resource().Attributes().AsRaw(), rm2.Resource().Attributes().AsRaw())

		ilms1 := rm1.InstrumentationLibraryMetrics()
		ilms2 := rm2.InstrumentationLibraryMetrics()

		if ilms1.Len() != ilms2.Len() {
			require.FailNow(t, "Resource metric %d: First metric had %d InstrumentationLibrary metrics, second had %d", i, ilms1.Len(), ilms2.Len())
		}

		for j := 0; j < ilms1.Len(); j++ {
			ilm1 := ilms1.At(j)
			ilm2 := ilms2.At(j)

			require.Equal(t, ilm1.InstrumentationLibrary().Name(), ilm2.InstrumentationLibrary().Name())

			err := scrapertest.CompareMetricSlices(ilm1.Metrics(), ilm2.Metrics())
			require.NoError(t, err)
		}
	}
}
