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

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const fullExpectedMetricsPath = "./testdata/expected_metrics/full.json"
const partialExpectedMetricsPath = "./testdata/expected_metrics/mostly_disabled.json"
const allQueryMetrics = "./testdata/mocked_queries/all_query_results.json"
const mostlyDisabledQueryMetrics = "./testdata/mocked_queries/mostly_disabled_results.json"

func TestScraper(t *testing.T) {
	t.Parallel()

	dbWrapper := &testDBWrapper{}
	initializeWrapper(t, dbWrapper, allQueryMetrics)

	sc, err := newSapHanaScraper(receivertest.NewNopCreateSettings(), createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(fullExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.Scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestDisabledMetrics(t *testing.T) {
	t.Parallel()

	dbWrapper := &testDBWrapper{}
	initializeWrapper(t, dbWrapper, mostlyDisabledQueryMetrics)

	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaAlertCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaBackupLatest.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaColumnMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaComponentMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaConnectionCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaCPUUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaDiskSizeCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaHostMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaHostSwapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaInstanceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaInstanceMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaInstanceMemorySharedAllocated.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaInstanceMemoryUsedPeak.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaLicenseExpirationTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaLicenseLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaLicensePeak.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaNetworkRequestAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaNetworkRequestCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaNetworkRequestFinishedCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaReplicationAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaReplicationBacklogSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaReplicationBacklogTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaRowStoreMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaSchemaMemoryUsedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaSchemaMemoryUsedMax.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaSchemaOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaSchemaRecordCompressedCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaSchemaRecordCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceCount.Enabled = true // Service Count Enabled
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryCompactorsAllocated.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryCompactorsFreeable.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryEffectiveLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryHeapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemorySharedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceStackSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaServiceThreadCount.Enabled = true // Service Thread Count Enabled
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaTransactionBlocked.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaTransactionCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaUptime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaVolumeOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaVolumeOperationSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettings.SaphanaVolumeOperationTime.Enabled = false

	sc, err := newSapHanaScraper(receivertest.NewNopCreateSettings(), cfg, &testConnectionFactory{dbWrapper})
	require.NoError(t, err)

	expectedMetrics, err := golden.ReadMetrics(partialExpectedMetricsPath)
	require.NoError(t, err)

	actualMetrics, err := sc.Scrape(context.Background())
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

type queryJSON struct {
	Query  string
	Result [][]string
}

func initializeWrapper(t *testing.T, w *testDBWrapper, filename string) {
	w.On("PingContext").Return(nil)
	w.On("Close").Return(nil)

	contents, err := os.ReadFile(filename)
	require.NoError(t, err)

	var queries []queryJSON
	err = json.Unmarshal(contents, &queries)
	require.NoError(t, err)

	for _, query := range queries {
		var result [][]*string
		for _, providedRow := range query.Result {
			var row []*string
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
