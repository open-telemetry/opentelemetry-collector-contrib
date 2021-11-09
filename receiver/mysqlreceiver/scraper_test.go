// Copyright  OpenTelemetry Authors
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

package mysqlreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
)

func TestScrape(t *testing.T) {
	mysqlMock := fakeClient{}
	sc := newMySQLScraper(zap.NewNop(), &Config{
		Username: "otel",
		Password: "otel",
		NetAddr: confignet.NetAddr{
			Endpoint: "localhost:3306",
		},
	})
	sc.sqlclient = &mysqlMock

	scrapedRMS, err := sc.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := scrapertest.ReadExpected(expectedFile)
	require.NoError(t, err)

	eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	aMetricSlice := scrapedRMS.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	require.NoError(t, scrapertest.CompareMetricSlices(eMetricSlice, aMetricSlice))
}
