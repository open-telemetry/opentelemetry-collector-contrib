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
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaAlertCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaBackupLatest.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaColumnMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaComponentMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaConnectionCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaCPUUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaDiskSizeCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaHostMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaHostSwapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaInstanceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaInstanceMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaInstanceMemorySharedAllocated.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaInstanceMemoryUsedPeak.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaLicenseExpirationTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaLicenseLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaLicensePeak.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaNetworkRequestAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaNetworkRequestCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaNetworkRequestFinishedCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaReplicationAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaReplicationBacklogSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaReplicationBacklogTime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaRowStoreMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaSchemaMemoryUsedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaSchemaMemoryUsedMax.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaSchemaOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaSchemaRecordCompressedCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaSchemaRecordCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceCount.Enabled = true // Service Count Enabled
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryCompactorsAllocated.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryCompactorsFreeable.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryEffectiveLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryHeapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryLimit.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemorySharedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceStackSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaServiceThreadCount.Enabled = true // Service Thread Count Enabled
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaTransactionBlocked.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaTransactionCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaUptime.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaVolumeOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaVolumeOperationSize.Enabled = false
	cfg.MetricsBuilderConfig.MetricsSettingsSaphanaVolumeOperationTime.Enabled = false

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
