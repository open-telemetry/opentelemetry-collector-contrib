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

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

const fullExpectedMetricsPath = "./testdata/expected_metrics/full.json"
const basicQueryMetrics = "./testdata/mocked_query_results.json"

func TestScraper(t *testing.T) {
	t.Parallel()

	dbWrapper := testDBWrapper{}
	initializeWrapper(t, &dbWrapper, basicQueryMetrics)

	sc, err := newSapHanaScraper(componenttest.NewNopTelemetrySettings(), createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(fullExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.Scrape(context.Background())
	require.NoError(t, err)

	scrapertest.CompareMetrics(expectedMetrics, actualMetrics)
}

type queryJson struct {
	Query  string
	Result [][]string
}

func initializeWrapper(t *testing.T, w *testDBWrapper, filename string) {
	w.On("PingContext").Return(nil)
	w.On("Close").Return(nil)

	contents, err := ioutil.ReadFile(filename)
	require.NoError(t, err)

	var queries []queryJson
	err = json.Unmarshal(contents, &queries)
	require.NoError(t, err)

	for _, query := range queries {
		result := [][]*string{}
		for _, providedRow := range query.Result {
			row := []*string{}
			for _, val := range providedRow {
				if val == "nil" {
					row = append(row, nil)
				} else {
					row = append(row, str(val))
				}
			}
			result = append(result, row)
		}

		w.mockQueryResult(query.Query, result, nil)
	}
}
