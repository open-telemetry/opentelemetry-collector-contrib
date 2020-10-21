// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudpubsubexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

var (
	tsRef       = time.Date(2022, 11, 28, 12, 0, 0, 0, time.UTC)
	tsBefore30s = tsRef.Add(-30 * time.Second)
	tsBefore1m  = tsRef.Add(-1 * time.Minute)
	tsBefore5m  = tsRef.Add(-5 * time.Minute)
	tsAfter30s  = tsRef.Add(30 * time.Second)
	tsAfter5m   = tsRef.Add(5 * time.Minute)
)

var metricsData = func() pdata.Metrics {
	d := pdata.NewMetrics()
	metric := d.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.Histogram().DataPoints().AppendEmpty().SetTimestamp(pdata.NewTimestampFromTime(tsAfter30s))
	metric = d.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeSummary)
	metric.Summary().DataPoints().AppendEmpty().SetTimestamp(pdata.NewTimestampFromTime(tsAfter5m))
	metric = d.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.Gauge().DataPoints().AppendEmpty().SetTimestamp(pdata.NewTimestampFromTime(tsRef))
	metric = d.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().DataPoints().AppendEmpty().SetTimestamp(pdata.NewTimestampFromTime(tsBefore30s))
	metric = d.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeExponentialHistogram)
	metric.ExponentialHistogram().DataPoints().AppendEmpty().SetTimestamp(pdata.NewTimestampFromTime(tsBefore5m))
	return d
}()

var tracesData = func() pdata.Traces {
	d := pdata.NewTraces()
	span := d.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetStartTimestamp(pdata.NewTimestampFromTime(tsRef))
	span = d.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetStartTimestamp(pdata.NewTimestampFromTime(tsBefore30s))
	span = d.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetStartTimestamp(pdata.NewTimestampFromTime(tsBefore5m))
	return d
}()

var logsData = func() pdata.Logs {
	d := pdata.NewLogs()
	log := d.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	log.SetTimestamp(pdata.NewTimestampFromTime(tsRef))
	log = d.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	log.SetTimestamp(pdata.NewTimestampFromTime(tsBefore30s))
	log = d.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	log.SetTimestamp(pdata.NewTimestampFromTime(tsBefore5m))
	return d
}()

func TestCurrentMetricsWatermark(t *testing.T) {
	out := currentMetricsWatermark(metricsData, tsRef, time.Minute)
	assert.Equal(t, tsRef, out)
}

func TestCurrentTracesWatermark(t *testing.T) {
	out := currentTracesWatermark(tracesData, tsRef, time.Minute)
	assert.Equal(t, tsRef, out)
}

func TestCurrentLogsWatermark(t *testing.T) {
	out := currentLogsWatermark(logsData, tsRef, time.Minute)
	assert.Equal(t, tsRef, out)
}

func TestEarliestMetricsWatermarkInDrift(t *testing.T) {
	out := earliestMetricsWatermark(metricsData, tsRef, time.Hour)
	assert.Equal(t, tsBefore5m, out)
}

func TestEarliestMetricsWatermarkOutDrift(t *testing.T) {
	out := earliestMetricsWatermark(metricsData, tsRef, time.Minute)
	assert.Equal(t, tsBefore1m, out)
}

func TestEarliestLogsWatermarkInDrift(t *testing.T) {
	out := earliestLogsWatermark(logsData, tsRef, time.Hour)
	assert.Equal(t, tsBefore5m, out)
}

func TestEarliestLogsWatermarkOutDrift(t *testing.T) {
	out := earliestLogsWatermark(logsData, tsRef, time.Minute)
	assert.Equal(t, tsBefore1m, out)
}

func TestEarliestTracessWatermarkInDrift(t *testing.T) {
	out := earliestTracesWatermark(tracesData, tsRef, time.Hour)
	assert.Equal(t, tsBefore5m, out)
}

func TestEarliestTracesWatermarkOutDrift(t *testing.T) {
	out := earliestTracesWatermark(tracesData, tsRef, time.Minute)
	assert.Equal(t, tsBefore1m, out)
}
