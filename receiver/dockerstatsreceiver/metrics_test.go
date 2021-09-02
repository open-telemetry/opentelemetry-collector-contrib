// Copyright 2020 OpenTelemetry Authors
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

package dockerstatsreceiver

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

type MetricType int32

const (
	MetricTypeCumulative MetricType = iota
	MetricTypeGauge
	MetricTypeDoubleGauge
)

type Metric struct {
	name      string
	mtype     MetricType
	unit      string
	labelKeys []string
	values    []Value
}

type Value struct {
	labelValues []string
	value       int64
	doubleValue float64
}

func metricsData(
	ts pdata.Timestamp,
	resourceLabels map[string]string,
	metrics ...Metric,
) pdata.Metrics {
	rLabels := mergeMaps(defaultLabels(), resourceLabels)
	md := pdata.NewMetrics()
	rs := md.ResourceMetrics().AppendEmpty()
	rsAttr := rs.Resource().Attributes()
	for k, v := range rLabels {
		rsAttr.UpsertString(k, v)
	}
	rsAttr.Sort()

	mdMetrics := rs.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
	mdMetrics.EnsureCapacity(len(metrics))
	for _, m := range metrics {
		mdMetric := mdMetrics.AppendEmpty()
		mdMetric.SetName(m.name)
		mdMetric.SetUnit(m.unit)

		var dps pdata.NumberDataPointSlice
		switch m.mtype {
		case MetricTypeCumulative:
			mdMetric.SetDataType(pdata.MetricDataTypeSum)
			mdMetric.Sum().SetIsMonotonic(true)
			mdMetric.Sum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
			dps = mdMetric.Sum().DataPoints()
		case MetricTypeGauge, MetricTypeDoubleGauge:
			mdMetric.SetDataType(pdata.MetricDataTypeGauge)
			dps = mdMetric.Gauge().DataPoints()
		}

		for _, v := range m.values {
			dp := dps.AppendEmpty()
			dp.SetTimestamp(ts)
			if m.mtype == MetricTypeDoubleGauge {
				dp.SetDoubleVal(v.doubleValue)
			} else {
				dp.SetIntVal(v.value)
			}
			populateAttributes(dp.Attributes(), m.labelKeys, v.labelValues)
		}
	}

	return md
}

func defaultLabels() map[string]string {
	return map[string]string{
		"container.hostname":   "abcdef012345",
		"container.id":         "a2596076ca048f02bcd16a8acd12a7ea2d3bc430d1cde095357239dd3925a4c3",
		"container.image.name": "myImage",
		"container.name":       "my-container-name",
	}
}

func defaultMetrics() []Metric {
	return []Metric{
		{name: "container.blockio.io_service_bytes_recursive.read", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 56500224}}},
		{name: "container.blockio.io_service_bytes_recursive.write", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 12103680}}},
		{name: "container.blockio.io_service_bytes_recursive.sync", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 65314816}}},
		{name: "container.blockio.io_service_bytes_recursive.async", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3289088}}},
		{name: "container.blockio.io_service_bytes_recursive.discard", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_service_bytes_recursive.total", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 68603904}}},
		{name: "container.blockio.io_serviced_recursive.read", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 985}}},
		{name: "container.blockio.io_serviced_recursive.write", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2073}}},
		{name: "container.blockio.io_serviced_recursive.sync", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2902}}},
		{name: "container.blockio.io_serviced_recursive.async", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 156}}},
		{name: "container.blockio.io_serviced_recursive.discard", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_serviced_recursive.total", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3058}}},
		{name: "container.cpu.usage.system", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 4525290000000}}},
		{name: "container.cpu.usage.total", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 8043152341}}},
		{name: "container.cpu.usage.kernelmode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 970000000}}},
		{name: "container.cpu.usage.usermode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 3510000000}}},
		{name: "container.cpu.throttling_data.periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0.19316}}},
		{name: "container.memory.usage.limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 1026359296}}},
		{name: "container.memory.usage.total", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 75915264}}},
		{name: "container.memory.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 7.396558329608582}}},
		{name: "container.memory.usage.max", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 325246976}}},
		{name: "container.memory.active_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.active_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.cache", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.dirty", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.hierarchical_memory_limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 9223372036854771712}}},
		{name: "container.memory.hierarchical_memsw_limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.mapped_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.pgfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.pgmajfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.pgpgin", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.pgpgout", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.rss", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.rss_huge", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_active_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.total_active_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.total_cache", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.total_dirty", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.total_mapped_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.total_pgfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.total_pgmajfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.total_pgpgin", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.total_pgpgout", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.total_rss", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.total_rss_huge", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_unevictable", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_writeback", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.unevictable", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.writeback", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.network.io.usage.rx_bytes", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2787669}}},
		{name: "container.network.io.usage.tx_bytes", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2275281}}},
		{name: "container.network.io.usage.rx_dropped", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_errors", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_packets", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 16598}}},
		{name: "container.network.io.usage.tx_dropped", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_errors", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_packets", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 9050}}},
	}
}

func mergeMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func assertMetricsDataEqual(
	t *testing.T,
	now pdata.Timestamp,
	expected []Metric,
	labels map[string]string,
	actual pdata.Metrics,
) {
	actual.ResourceMetrics().At(0).Resource().Attributes().Sort()
	assert.Equal(t, metricsData(now, labels, expected...), actual)
}

func TestZeroValueStats(t *testing.T) {
	stats := &dtypes.StatsJSON{
		Stats: dtypes.Stats{
			BlkioStats:  dtypes.BlkioStats{},
			CPUStats:    dtypes.CPUStats{},
			MemoryStats: dtypes.MemoryStats{},
		},
		Networks: nil,
	}
	containers := containerJSON(t)
	config := &Config{}

	now := pdata.NewTimestampFromTime(time.Now())
	md, err := ContainerStatsToMetrics(now, stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	metrics := []Metric{
		{name: "container.cpu.usage.system", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.total", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.kernelmode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.usermode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0}}},
		{name: "container.memory.usage.limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.usage.total", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0}}},
		{name: "container.memory.usage.max", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
	}
	assertMetricsDataEqual(t, now, metrics, nil, md)
}

func statsJSON(t *testing.T) *dtypes.StatsJSON {
	statsRaw, err := ioutil.ReadFile(path.Join(".", "testdata", "stats.json"))
	if err != nil {
		t.Fatal(err)
	}

	var stats dtypes.StatsJSON
	err = json.Unmarshal(statsRaw, &stats)
	if err != nil {
		t.Fatal(err)
	}
	return &stats
}

func containerJSON(t *testing.T) *DockerContainer {
	containerRaw, err := ioutil.ReadFile(path.Join(".", "testdata", "container.json"))
	if err != nil {
		t.Fatal(err)
	}

	var container dtypes.ContainerJSON
	err = json.Unmarshal(containerRaw, &container)
	if err != nil {
		t.Fatal(err)
	}
	return &DockerContainer{
		ContainerJSON: &container,
		EnvMap:        containerEnvToMap(container.Config.Env),
	}
}

func TestStatsToDefaultMetrics(t *testing.T) {
	stats := statsJSON(t)
	containers := containerJSON(t)
	config := &Config{}

	now := pdata.NewTimestampFromTime(time.Now())
	md, err := ContainerStatsToMetrics(now, stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	assertMetricsDataEqual(t, now, defaultMetrics(), nil, md)
}

func TestStatsToAllMetrics(t *testing.T) {
	stats := statsJSON(t)
	containers := containerJSON(t)
	config := &Config{
		ProvidePerCoreCPUMetrics: true,
	}

	now := pdata.NewTimestampFromTime(time.Now())
	md, err := ContainerStatsToMetrics(now, stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	metrics := []Metric{
		{name: "container.blockio.io_service_bytes_recursive.read", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 56500224}}},
		{name: "container.blockio.io_service_bytes_recursive.write", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 12103680}}},
		{name: "container.blockio.io_service_bytes_recursive.sync", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 65314816}}},
		{name: "container.blockio.io_service_bytes_recursive.async", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3289088}}},
		{name: "container.blockio.io_service_bytes_recursive.discard", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_service_bytes_recursive.total", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 68603904}}},
		{name: "container.blockio.io_serviced_recursive.read", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 985}}},
		{name: "container.blockio.io_serviced_recursive.write", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2073}}},
		{name: "container.blockio.io_serviced_recursive.sync", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2902}}},
		{name: "container.blockio.io_serviced_recursive.async", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 156}}},
		{name: "container.blockio.io_serviced_recursive.discard", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_serviced_recursive.total", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3058}}},
		{name: "container.cpu.usage.system", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 4525290000000}}},
		{name: "container.cpu.usage.total", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 8043152341}}},
		{name: "container.cpu.usage.kernelmode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 970000000}}},
		{name: "container.cpu.usage.usermode", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 3510000000}}},
		{name: "container.cpu.throttling_data.periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: MetricTypeCumulative, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0.19316}}},
		{
			name:      "container.cpu.usage.percpu",
			mtype:     MetricTypeCumulative,
			unit:      "ns",
			labelKeys: []string{"core"},
			values: []Value{
				{labelValues: []string{"cpu0"}, value: 8043152341},
				{labelValues: []string{"cpu1"}, value: 0},
				{labelValues: []string{"cpu2"}, value: 0},
				{labelValues: []string{"cpu3"}, value: 0},
				{labelValues: []string{"cpu4"}, value: 0},
				{labelValues: []string{"cpu5"}, value: 0},
				{labelValues: []string{"cpu6"}, value: 0},
				{labelValues: []string{"cpu7"}, value: 0},
			},
		},
		{name: "container.memory.usage.limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 1026359296}}},
		{name: "container.memory.usage.total", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 75915264}}},
		{name: "container.memory.percent", mtype: MetricTypeDoubleGauge, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 7.396558329608582}}},
		{name: "container.memory.usage.max", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 325246976}}},
		{name: "container.memory.active_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.active_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.cache", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.dirty", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.hierarchical_memory_limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 9223372036854771712}}},
		{name: "container.memory.hierarchical_memsw_limit", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.mapped_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.pgfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.pgmajfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.pgpgin", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.pgpgout", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.rss", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.rss_huge", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_active_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.total_active_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.total_cache", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.total_dirty", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_anon", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.total_mapped_file", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.total_pgfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.total_pgmajfault", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.total_pgpgin", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.total_pgpgout", mtype: MetricTypeCumulative, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.total_rss", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.total_rss_huge", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_unevictable", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_writeback", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.unevictable", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.writeback", mtype: MetricTypeGauge, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.network.io.usage.rx_bytes", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2787669}}},
		{name: "container.network.io.usage.tx_bytes", mtype: MetricTypeCumulative, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2275281}}},
		{name: "container.network.io.usage.rx_dropped", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_errors", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_packets", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 16598}}},
		{name: "container.network.io.usage.tx_dropped", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_errors", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_packets", mtype: MetricTypeCumulative, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 9050}}},
	}

	assertMetricsDataEqual(t, now, metrics, nil, md)
}

func TestEnvVarToMetricLabels(t *testing.T) {
	stats := statsJSON(t)
	containers := containerJSON(t)
	config := &Config{
		EnvVarsToMetricLabels: map[string]string{
			"MY_ENV_VAR":       "my.env.to.metric.label",
			"MY_OTHER_ENV_VAR": "my.other.env.to.metric.label",
		},
	}

	now := pdata.NewTimestampFromTime(time.Now())
	md, err := ContainerStatsToMetrics(now, stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	expectedLabels := map[string]string{
		"my.env.to.metric.label":       "my_env_var_value",
		"my.other.env.to.metric.label": "my_other_env_var_value",
	}

	assertMetricsDataEqual(t, now, defaultMetrics(), expectedLabels, md)
}

func TestContainerLabelToMetricLabels(t *testing.T) {
	stats := statsJSON(t)
	containers := containerJSON(t)
	config := &Config{
		ContainerLabelsToMetricLabels: map[string]string{
			"my.specified.docker.label":    "my.docker.to.metric.label",
			"other.specified.docker.label": "my.other.docker.to.metric.label",
		},
	}

	now := pdata.NewTimestampFromTime(time.Now())
	md, err := ContainerStatsToMetrics(now, stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	expectedLabels := map[string]string{
		"my.docker.to.metric.label":       "my_specified_docker_label_value",
		"my.other.docker.to.metric.label": "other_specified_docker_label_value",
	}

	assertMetricsDataEqual(t, now, defaultMetrics(), expectedLabels, md)
}
