// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:           "",
		ExternalLabels:      nil,
		DisableTargetInfo:   false,
		ExportCreatedMetric: false,
		AddMetricSuffixes:   false,
		SendMetadata:        false,
	}

	ts := uint64(time.Now().UnixNano())
	payload := createExportRequest(5, 0, 1, 3, 0, pcommon.Timestamp(ts))
	want := func() map[string]*writev2.TimeSeries {
		return map[string]*writev2.TimeSeries{
			"0": {
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
				Samples: []writev2.Sample{
					{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
				},
			},
		}
	}
	wantedSymbols := []string{"", "series_name_2", "value-2", "series_name_3", "value-3", "__name__", "gauge_1", "series_name_1", "value-1"}
	tsMap, symbolsTable, err := FromMetricsV2(payload.Metrics(), settings)
	require.NoError(t, err)
	require.Equal(t, want(), tsMap)
	require.ElementsMatch(t, wantedSymbols, symbolsTable.Symbols())
}
