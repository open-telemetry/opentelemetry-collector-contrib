// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

const fullExpectedMetricsPath = "./testdata/expected_metrics/full.yaml"
const partialExpectedMetricsPath = "./testdata/expected_metrics/mostly_disabled.yaml"
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
		pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestDisabledMetrics(t *testing.T) {
	t.Parallel()

	dbWrapper := &testDBWrapper{}
	initializeWrapper(t, dbWrapper, mostlyDisabledQueryMetrics)

	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.Metrics.SaphanaAlertCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaBackupLatest.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaColumnMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaComponentMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaConnectionCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaCPUUsed.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaDiskSizeCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaHostMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaHostSwapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaInstanceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaInstanceMemoryCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaInstanceMemorySharedAllocated.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaInstanceMemoryUsedPeak.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaLicenseExpirationTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaLicenseLimit.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaLicensePeak.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestFinishedCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaReplicationAverageTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaReplicationBacklogSize.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaReplicationBacklogTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaRowStoreMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaSchemaMemoryUsedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaSchemaMemoryUsedMax.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaSchemaOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaSchemaRecordCompressedCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaSchemaRecordCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceCodeSize.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceCount.Enabled = true // Service Count Enabled
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryCompactorsAllocated.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryCompactorsFreeable.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryEffectiveLimit.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryHeapCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryLimit.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemorySharedCurrent.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryUsed.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceStackSize.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaServiceThreadCount.Enabled = true // Service Thread Count Enabled
	cfg.MetricsBuilderConfig.Metrics.SaphanaTransactionBlocked.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaTransactionCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaUptime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationSize.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationTime.Enabled = false

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
