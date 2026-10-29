// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"
)

// TestProcessSubtaskMetricsOperatorSuffixMatch checks that operator metrics are
// matched on their exact suffix. Flink also exposes rate metrics such as
// "<operator>.numRecordsInPerSecond", which must not be recorded as the
// numRecordsIn counter even when their value parses as an integer.
func TestProcessSubtaskMetricsOperatorSuffixMatch(t *testing.T) {
	var metrics models.MetricsResponse
	require.NoError(t, json.Unmarshal([]byte(`[
		{"id": "GlobalWindowAggregate[6].numRecordsIn", "value": "10"},
		{"id": "GlobalWindowAggregate[6].numRecordsInPerSecond", "value": "7"},
		{"id": "GlobalWindowAggregate[6].numRecordsOut", "value": "5"},
		{"id": "GlobalWindowAggregate[6].numRecordsOutPerSecond", "value": "3"},
		{"id": "GlobalWindowAggregate[6].numLateRecordsDropped", "value": "2"},
		{"id": "GlobalWindowAggregate[6].currentOutputWatermark", "value": "1000"},
		{"id": "GlobalWindowAggregate[6].currentInputWatermark", "value": "900"}
	]`), &metrics))

	scraper := newflinkScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(metadata.Type))
	scraper.processSubtaskMetrics(pcommon.NewTimestampFromTime(time.Now()), []*models.SubtaskMetrics{{
		Host:          "localhost",
		TaskmanagerID: "localhost:39799-811290",
		JobName:       "job",
		TaskName:      "GlobalWindowAggregate[6] -> Calc[7] -> sink[8]: Writer",
		SubtaskIndex:  "0",
		Metrics:       metrics,
	}})

	records := map[string]int64{}
	var watermarks []int64
	rms := scraper.mb.Emit().ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				switch m.Name() {
				case "flink.operator.record.count":
					dps := m.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						requireOperatorName(t, dp, "GlobalWindowAggregate[6]")
						record, ok := dp.Attributes().Get("record")
						require.True(t, ok)
						_, dup := records[record.Str()]
						require.False(t, dup, "duplicate data point for record=%s", record.Str())
						records[record.Str()] = dp.IntValue()
					}
				case "flink.operator.watermark.output":
					dps := m.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						requireOperatorName(t, dps.At(l), "GlobalWindowAggregate[6]")
						watermarks = append(watermarks, dps.At(l).IntValue())
					}
				}
			}
		}
	}

	require.Equal(t, map[string]int64{"in": 10, "out": 5, "dropped": 2}, records)
	require.Equal(t, []int64{1000}, watermarks)
}

func requireOperatorName(t *testing.T, dp pmetric.NumberDataPoint, want string) {
	t.Helper()
	name, ok := dp.Attributes().Get("name")
	require.True(t, ok)
	require.Equal(t, want, name.Str())
}
