// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestMemoryMetric(t *testing.T) {
	rm := testGetMetricData(t, usedMemory())

	const metricName = "redis/memory/used"
	const desc = "Total number of bytes allocated by Redis using its allocator"
	const units = "By"
	const ptVal = 854160

	ilms := rm.InstrumentationLibraryMetrics()
	assert.Equal(t, 1, ilms.Len())
	assert.Equal(t, 0, rm.Resource().Attributes().Len())
	ilm := ilms.At(0)
	ms := ilm.Metrics()
	m := ms.At(0)
	assert.Equal(t, "otelcol/redis", ilm.InstrumentationLibrary().Name())
	assert.Equal(t, metricName, m.Name())
	assert.Equal(t, desc, m.Description())
	assert.Equal(t, units, m.Unit())
	assert.Equal(t, pdata.MetricDataTypeGauge, m.DataType())
	assert.Equal(t, int64(ptVal), m.Gauge().DataPoints().At(0).IntVal())
}

func TestUptimeInSeconds(t *testing.T) {
	pdm := testGetMetric(t, uptimeInSeconds())
	const units = "s"
	v := 104946

	assert.Equal(t, units, pdm.Unit())
	assert.Equal(t, int64(v), pdm.Sum().DataPoints().At(0).IntVal())
}

func TestUsedCpuSys(t *testing.T) {
	pdm := testGetMetricData(t, usedCPUSys())
	const units = "s"
	v := 185.649184

	m := pdm.InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	assert.Equal(
		t,
		pdata.MetricDataTypeSum,
		m.DataType(),
	)
	assert.Equal(t, units, m.Unit())
	assert.Equal(t, v, m.Sum().DataPoints().At(0).DoubleVal())
}

func TestMissingMetricValue(t *testing.T) {
	redisMetrics := []*redisMetric{{key: "config_file"}}
	_, warnings, err := testFetchMetrics(redisMetrics)
	require.Nil(t, err)
	// treat a missing value as not worthy of a warning
	require.Nil(t, warnings)
}

func TestMissingMetric(t *testing.T) {
	// unlike the above test, the key "foo" not in the set of known keys
	// which should cause a warning
	redisMetrics := []*redisMetric{{key: "foo"}}
	_, warnings, err := testFetchMetrics(redisMetrics)
	require.NoError(t, err)
	require.Equal(t, 1, len(warnings))
}

func TestAllMetrics(t *testing.T) {
	redisMetrics := getDefaultRedisMetrics()
	ms, warnings, err := testFetchMetrics(redisMetrics)
	require.NoError(t, err)
	require.Nil(t, warnings)
	require.Equal(t, len(redisMetrics), ms.Len())
}

func TestKeyspaceMetrics(t *testing.T) {
	svc := newRedisSvc(newFakeClient())
	info, _ := svc.info()
	ms, errs := info.buildKeyspaceMetrics(testTimeBundle())
	require.Nil(t, errs)

	// 2 dbs * 3 metrics each
	assert.Equal(t, 6, ms.Len())

	const lblKey = "db"
	const name1 = "redis/db/keys"

	lblVal := pdata.NewAttributeValueString("0")

	pdm := ms.At(0)
	assert.Equal(t, name1, pdm.Name())
	dps := pdm.Gauge().DataPoints()
	pt := dps.At(0)
	v, ok := pt.Attributes().Get(lblKey)
	assert.True(t, ok)
	assert.Equal(t, lblVal, v)
	assert.Equal(t, pdata.MetricDataTypeGauge, pdm.DataType())
	assert.Equal(t, int64(1), pt.IntVal())

	const name2 = "redis/db/expires"

	pdm = ms.At(1)
	assert.Equal(t, name2, pdm.Name())
	dps = pdm.Gauge().DataPoints()
	pt = dps.At(0)
	v, ok = pt.Attributes().Get(lblKey)
	assert.True(t, ok)
	assert.Equal(t, lblVal, v)
	assert.Equal(t, pdata.MetricDataTypeGauge, pdm.DataType())
	assert.Equal(t, int64(2), pt.IntVal())

	const name3 = "redis/db/avg_ttl"

	pdm = ms.At(2)
	assert.Equal(t, name3, pdm.Name())
	dps = pdm.Gauge().DataPoints()
	pt = dps.At(0)
	v, ok = pt.Attributes().Get(lblKey)
	assert.True(t, ok)
	assert.Equal(t, lblVal, v)
	assert.Equal(t, pdata.MetricDataTypeGauge, pdm.DataType())
	assert.Equal(t, int64(3), pt.IntVal())
}

func TestNewPDM(t *testing.T) {
	serverStartTime := pdata.NewTimestampFromTime(time.Unix(900, 0))
	tb := testTimeBundle()

	pdm := pdata.NewMetric()
	initIntMetric(&redisMetric{pdType: pdata.MetricDataTypeGauge}, 0, tb, pdm)
	assert.Equal(t, pdata.Timestamp(0), pdm.Gauge().DataPoints().At(0).StartTimestamp())

	pdm = pdata.NewMetric()
	initIntMetric(&redisMetric{pdType: pdata.MetricDataTypeSum}, 0, tb, pdm)
	assert.Equal(t, serverStartTime, pdm.Sum().DataPoints().At(0).StartTimestamp())

	pdm = pdata.NewMetric()
	initDoubleMetric(&redisMetric{pdType: pdata.MetricDataTypeGauge}, 0, tb, pdm)
	assert.Equal(t, pdata.Timestamp(0), pdm.Gauge().DataPoints().At(0).StartTimestamp())

	pdm = pdata.NewMetric()
	initDoubleMetric(&redisMetric{pdType: pdata.MetricDataTypeSum}, 0, tb, pdm)
	assert.Equal(t, serverStartTime, pdm.Sum().DataPoints().At(0).StartTimestamp())
}

func newResourceMetrics(ms pdata.MetricSlice) pdata.ResourceMetrics {
	rm := pdata.NewResourceMetrics()
	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InstrumentationLibrary().SetName("otelcol/redis")
	tgt := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.CopyTo(tgt)
	ms.CopyTo(tgt.Metrics())
	return rm
}

func testFetchMetrics(redisMetrics []*redisMetric) (pdata.MetricSlice, []error, error) {
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	if err != nil {
		return pdata.MetricSlice{}, nil, err
	}
	ms, warnings := info.buildFixedMetrics(redisMetrics, testTimeBundle())
	return ms, warnings, nil
}

func testGetMetric(t *testing.T, redisMetric *redisMetric) pdata.Metric {
	rm := testGetMetricData(t, redisMetric)
	pdm := rm.InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	return pdm
}

func testGetMetricData(t *testing.T, metric *redisMetric) pdata.ResourceMetrics {
	rm, warnings, err := testGetMetricDataErr(metric)
	require.Nil(t, err)
	require.Nil(t, warnings)
	return rm
}

func testGetMetricDataErr(metric *redisMetric) (pdata.ResourceMetrics, []error, error) {
	redisMetrics := []*redisMetric{metric}
	svc := newRedisSvc(newFakeClient())
	info, err := svc.info()
	if err != nil {
		return pdata.ResourceMetrics{}, nil, err
	}
	ms, warnings := info.buildFixedMetrics(redisMetrics, testTimeBundle())
	rm := newResourceMetrics(ms)
	return rm, warnings, nil
}

func testTimeBundle() *timeBundle {
	return newTimeBundle(time.Unix(1000, 0), 100)
}
