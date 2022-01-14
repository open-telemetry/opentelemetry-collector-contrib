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
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.opentelemetry.io/collector/model/otlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScrape(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	adminStatus, err := loadAdminStatus()
	require.NoError(t, err)
	ss, err := loadServerStatus()
	require.NoError(t, err)
	dbStats, err := loadDBStats()
	require.NoError(t, err)

	fakeDatabaseName := "fakedatabase"
	extractor, err := newExtractor(Mongo40.String(), zap.NewNop())
	require.NoError(t, err)

	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mont.Close()

	mont.Run("scrape", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(
				primitive.E{
					Key: "databases",
					Value: []struct {
						Name string `bson:"name,omitempty"`
					}{
						{
							Name: fakeDatabaseName,
						},
					},
				},
			),
			mtest.CreateSuccessResponse(dbStats...),
			mtest.CreateSuccessResponse(adminStatus...),
			mtest.CreateSuccessResponse(ss...),
		)

		driver := mt.Client
		client := &mongodbClient{
			Client: driver,
			logger: zap.NewNop(),
		}
		scraper := mongodbScraper{
			client:    client,
			config:    cfg,
			logger:    zap.NewNop(),
			extractor: extractor,
		}

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)
		aMetricSlice := actualMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

		bytes, err := otlp.NewJSONMetricsMarshaler().MarshalMetrics(actualMetrics)
		require.NoError(t, err)
		err = ioutil.WriteFile("./testdata/scraper/expected.json", bytes, 0600)
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
		require.NoError(t, scrapertest.CompareMetricSlices(eMetricSlice, aMetricSlice))
	})
}
