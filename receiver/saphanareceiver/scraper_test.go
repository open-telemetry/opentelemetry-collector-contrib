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
const partialExpectedMetricsPath = "./testdata/expected_metrics/mostly_disabled.json"
const allQueryMetrics = "./testdata/mocked_queries/all_query_results.json"
const mostlyDisabledQueryMetrics = "./testdata/mocked_queries/mostly_disabled_results.json"

func TestScraper(t *testing.T) {
	t.Parallel()

	dbWrapper := &testDBWrapper{}
	initializeWrapper(t, dbWrapper, allQueryMetrics)

	sc, err := newSapHanaScraper(componenttest.NewNopReceiverCreateSettings(), createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(fullExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.Scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestDisabledMetrics(t *testing.T) {
	t.Parallel()

	dbWrapper := &testDBWrapper{}
	initializeWrapper(t, dbWrapper, mostlyDisabledQueryMetrics)

	cfg := createDefaultConfig().(*Config)
	cfg.Metrics.SaphanaAlertCount.Enabled = false
	cfg.Metrics.SaphanaBackupLatest.Enabled = false
	cfg.Metrics.SaphanaColumnMemoryUsed.Enabled = false
	cfg.Metrics.SaphanaComponentMemoryUsed.Enabled = false
	cfg.Metrics.SaphanaConnectionCount.Enabled = false
	cfg.Metrics.SaphanaCPUUsed.Enabled = false
	cfg.Metrics.SaphanaDiskSizeCurrent.Enabled = false
	cfg.Metrics.SaphanaHostMemoryCurrent.Enabled = false
	cfg.Metrics.SaphanaHostSwapCurrent.Enabled = false
	cfg.Metrics.SaphanaInstanceCodeSize.Enabled = false
	cfg.Metrics.SaphanaInstanceMemoryCurrent.Enabled = false
	cfg.Metrics.SaphanaInstanceMemorySharedAllocated.Enabled = false
	cfg.Metrics.SaphanaInstanceMemoryUsedPeak.Enabled = false
	cfg.Metrics.SaphanaLicenseExpirationTime.Enabled = false
	cfg.Metrics.SaphanaLicenseLimit.Enabled = false
	cfg.Metrics.SaphanaLicensePeak.Enabled = false
	cfg.Metrics.SaphanaNetworkRequestAverageTime.Enabled = false
	cfg.Metrics.SaphanaNetworkRequestCount.Enabled = false
	cfg.Metrics.SaphanaNetworkRequestFinishedCount.Enabled = false
	cfg.Metrics.SaphanaReplicationAverageTime.Enabled = false
	cfg.Metrics.SaphanaReplicationBacklogSize.Enabled = false
	cfg.Metrics.SaphanaReplicationBacklogTime.Enabled = false
	cfg.Metrics.SaphanaRowStoreMemoryUsed.Enabled = false
	cfg.Metrics.SaphanaSchemaMemoryUsedCurrent.Enabled = false
	cfg.Metrics.SaphanaSchemaMemoryUsedMax.Enabled = false
	cfg.Metrics.SaphanaSchemaOperationCount.Enabled = false
	cfg.Metrics.SaphanaSchemaRecordCompressedCount.Enabled = false
	cfg.Metrics.SaphanaSchemaRecordCount.Enabled = false
	cfg.Metrics.SaphanaServiceCodeSize.Enabled = false
	cfg.Metrics.SaphanaServiceCount.Enabled = true // Service Count Enabled
	cfg.Metrics.SaphanaServiceMemoryCompactorsAllocated.Enabled = false
	cfg.Metrics.SaphanaServiceMemoryCompactorsFreeable.Enabled = false
	cfg.Metrics.SaphanaServiceMemoryEffectiveLimit.Enabled = false
	cfg.Metrics.SaphanaServiceMemoryHeapCurrent.Enabled = false
	cfg.Metrics.SaphanaServiceMemoryLimit.Enabled = false
	cfg.Metrics.SaphanaServiceMemorySharedCurrent.Enabled = false
	cfg.Metrics.SaphanaServiceMemoryUsed.Enabled = false
	cfg.Metrics.SaphanaServiceStackSize.Enabled = false
	cfg.Metrics.SaphanaServiceThreadCount.Enabled = true // Service Thread Count Enabled
	cfg.Metrics.SaphanaTransactionBlocked.Enabled = false
	cfg.Metrics.SaphanaTransactionCount.Enabled = false
	cfg.Metrics.SaphanaUptime.Enabled = false
	cfg.Metrics.SaphanaVolumeOperationCount.Enabled = false
	cfg.Metrics.SaphanaVolumeOperationSize.Enabled = false
	cfg.Metrics.SaphanaVolumeOperationTime.Enabled = false

	sc, err := newSapHanaScraper(componenttest.NewNopReceiverCreateSettings(), cfg, &testConnectionFactory{dbWrapper})
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(partialExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.Scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

type queryJSON struct {
	Query  string
	Result [][]string
}

func initializeWrapper(t *testing.T, w *testDBWrapper, filename string) {
	w.On("PingContext").Return(nil)
	w.On("Close").Return(nil)

	contents, err := ioutil.ReadFile(filename)
	require.NoError(t, err)

	var queries []queryJSON
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
