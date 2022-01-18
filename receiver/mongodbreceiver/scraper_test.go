// // Copyright The OpenTelemetry Authors
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //      http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

package mongodbreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
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

	fakeDatabaseName := "fakedatabase"
	extractor, err := newExtractor(Mongo40.String(), zap.NewNop())
	require.NoError(t, err)

	fc := &fakeClient{}
	fc.On("Connect", mock.Anything).Return(nil)
	fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
	fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(ss, nil)
	fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
	fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(dbStats, nil)

	scraper := mongodbScraper{
		client:    fc,
		config:    cfg,
		logger:    zap.NewNop(),
		extractor: extractor,
	}

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)
	aMetricSlice := actualMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	require.NoError(t, scrapertest.CompareMetricSlices(eMetricSlice, aMetricSlice))
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

func TestStart(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	testCases := []struct {
		desc          string
		responses     []bson.D
		errorContains string
	}{
		{
			desc:          "unable to get version response",
			responses:     []bson.D{mtest.CreateCommandErrorResponse(mtest.CommandError{})},
			errorContains: "unable to get a version from the mongo instance",
		},
		{
			desc: "incompatible version response",
			responses: []bson.D{{
				primitive.E{Key: "version", Value: "-9129;093"},
				primitive.E{Key: "ok", Value: 1},
			}},
			errorContains: "unable to parse version as semantic",
		},
	}

	for _, tc := range testCases {
		mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
		defer mont.Close()
		mont.Run("start", func(mt *mtest.T) {
			mt.AddMockResponses(tc.responses...)
			c := defaultTestClient(mt)
			scraper := mongodbScraper{
				client: c,
				config: cfg,
				logger: zap.NewNop(),
			}
			err := scraper.start(context.Background(), nil)
			if tc.errorContains != "" {
				require.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func defaultTestClient(mt *mtest.T) client {
	driver := mt.Client
	return &mongodbClient{
		Client: driver,
		logger: zap.NewNop(),
	}
}
