// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"encoding/json"
	"strconv"
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

// TestProcessGarbageCollectorMetrics checks that JVM garbage collector metrics are
// recorded for both the JobManager and the TaskManagers, whichever of the
// supported collectors the JVM uses. Flink exposes the same
// Status.JVM.GarbageCollector.* metrics for both components. The collector
// names are the ones Flink 1.20.5 (Java 17) and Flink 2.3.0 (Java 21) report.
func TestProcessGarbageCollectorMetrics(t *testing.T) {
	collectors := []struct {
		name  string
		names []string
	}{
		{name: "G1", names: []string{"G1_Young_Generation", "G1_Old_Generation"}},
		{name: "G1_Java21", names: []string{"G1_Young_Generation", "G1_Old_Generation", "G1_Concurrent_GC"}},
		{name: "Parallel", names: []string{"PS_Scavenge", "PS_MarkSweep"}},
		{name: "Serial", names: []string{"Copy", "MarkSweepCompact"}},
		{name: "ZGC", names: []string{"ZGC_Cycles", "ZGC_Pauses"}},
		{name: "ZGC_generational", names: []string{"ZGC_Minor_Cycles", "ZGC_Minor_Pauses", "ZGC_Major_Cycles", "ZGC_Major_Pauses"}},
		{name: "Shenandoah", names: []string{"Shenandoah_Cycles", "Shenandoah_Pauses"}},
	}
	components := []struct {
		name    string
		process func(*flinkmetricsScraper, pcommon.Timestamp, models.MetricsResponse)
	}{
		{name: "jobmanager", process: func(s *flinkmetricsScraper, now pcommon.Timestamp, metrics models.MetricsResponse) {
			s.processJobmanagerMetrics(now, &models.JobmanagerMetrics{Host: "localhost", Metrics: metrics})
		}},
		{name: "taskmanager", process: func(s *flinkmetricsScraper, now pcommon.Timestamp, metrics models.MetricsResponse) {
			s.processTaskmanagerMetrics(now, []*models.TaskmanagerMetrics{{Host: "localhost", TaskmanagerID: "localhost:39799-811290", Metrics: metrics}})
		}},
	}

	for _, component := range components {
		for _, collector := range collectors {
			names, process := collector.names, component.process
			t.Run(component.name+"/"+collector.name, func(t *testing.T) {
				raw := make([]map[string]string, 0, 3*len(names)+2)
				wantCount := map[string]int64{}
				wantTime := map[string]int64{}
				for i, name := range names {
					count := int64(11 * (i + 1))
					raw = append(raw,
						map[string]string{"id": "Status.JVM.GarbageCollector." + name + ".Count", "value": strconv.FormatInt(count, 10)},
						map[string]string{"id": "Status.JVM.GarbageCollector." + name + ".Time", "value": strconv.FormatInt(count*10, 10)},
						map[string]string{"id": "Status.JVM.GarbageCollector." + name + ".TimeMsPerSecond", "value": "1"},
					)
					wantCount[name] = count
					wantTime[name] = count * 10
				}
				raw = append(raw,
					map[string]string{"id": "Status.JVM.GarbageCollector.All.Count", "value": "999"},
					map[string]string{"id": "Status.JVM.GarbageCollector.All.Time", "value": "9990"},
				)
				body, err := json.Marshal(raw)
				require.NoError(t, err)
				var metrics models.MetricsResponse
				require.NoError(t, json.Unmarshal(body, &metrics))

				scraper := newflinkScraper(createDefaultConfig().(*Config), receivertest.NewNopSettings(metadata.Type))
				process(scraper, pcommon.NewTimestampFromTime(time.Now()), metrics)

				require.Equal(t, map[string]map[string]int64{
					"flink.jvm.gc.collections.count": wantCount,
					"flink.jvm.gc.collections.time":  wantTime,
				}, garbageCollectorDataPoints(t, scraper.mb.Emit()))
			})
		}
	}
}

// garbageCollectorDataPoints returns the flink.jvm.gc.* data points as
// metric name -> collector name -> value.
func garbageCollectorDataPoints(t *testing.T, md pmetric.Metrics) map[string]map[string]int64 {
	t.Helper()
	got := map[string]map[string]int64{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != "flink.jvm.gc.collections.count" && m.Name() != "flink.jvm.gc.collections.time" {
					continue
				}
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					name, ok := dps.At(l).Attributes().Get("name")
					require.True(t, ok)
					if got[m.Name()] == nil {
						got[m.Name()] = map[string]int64{}
					}
					_, dup := got[m.Name()][name.Str()]
					require.False(t, dup, "duplicate %s data point for %s", m.Name(), name.Str())
					got[m.Name()][name.Str()] = dps.At(l).IntValue()
				}
			}
		}
	}
	return got
}

func requireOperatorName(t *testing.T, dp pmetric.NumberDataPoint, want string) {
	t.Helper()
	name, ok := dp.Attributes().Get("name")
	require.True(t, ok)
	require.Equal(t, want, name.Str())
}
