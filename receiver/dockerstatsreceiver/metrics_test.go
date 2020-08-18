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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	dtypes "github.com/docker/docker/api/types"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

type Metric struct {
	name      string
	mtype     metricspb.MetricDescriptor_Type
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
	ts *timestamp.Timestamp,
	resourceLabels map[string]string,
	metrics ...Metric,
) *consumerdata.MetricsData {
	rlabels := mergeMaps(defaultLabels(), resourceLabels)
	md := &consumerdata.MetricsData{
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: rlabels,
		},
	}

	var mdMetrics []*metricspb.Metric
	for _, m := range metrics {
		labelKeys := make([]*metricspb.LabelKey, len(m.labelKeys))
		for i, k := range m.labelKeys {
			labelKeys[i] = &metricspb.LabelKey{Key: k}
		}

		metric := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:      m.name,
				Unit:      m.unit,
				Type:      m.mtype,
				LabelKeys: labelKeys,
			},
		}

		var timeseries []*metricspb.TimeSeries
		for _, v := range m.values {
			seriesItem := &metricspb.TimeSeries{
				Points: []*metricspb.Point{{Timestamp: ts}},
			}

			if len(v.labelValues) != 0 {
				labelValues := make([]*metricspb.LabelValue, len(v.labelValues))
				for i, k := range v.labelValues {
					labelValues[i] = &metricspb.LabelValue{Value: k, HasValue: true}
				}
				seriesItem.LabelValues = labelValues
			}

			if m.mtype == metricspb.MetricDescriptor_GAUGE_DOUBLE {
				seriesItem.Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: v.doubleValue}
			} else {
				seriesItem.Points[0].Value = &metricspb.Point_Int64Value{Int64Value: v.value}
			}

			timeseries = append(timeseries, seriesItem)
		}
		metric.Timeseries = timeseries
		mdMetrics = append(mdMetrics, metric)
	}

	md.Metrics = mdMetrics
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
		{name: "container.blockio.io_service_bytes_recursive.read", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 56500224}}},
		{name: "container.blockio.io_service_bytes_recursive.write", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 12103680}}},
		{name: "container.blockio.io_service_bytes_recursive.sync", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 65314816}}},
		{name: "container.blockio.io_service_bytes_recursive.async", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3289088}}},
		{name: "container.blockio.io_service_bytes_recursive.discard", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_service_bytes_recursive.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 68603904}}},
		{name: "container.blockio.io_serviced_recursive.read", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 985}}},
		{name: "container.blockio.io_serviced_recursive.write", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2073}}},
		{name: "container.blockio.io_serviced_recursive.sync", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2902}}},
		{name: "container.blockio.io_serviced_recursive.async", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 156}}},
		{name: "container.blockio.io_serviced_recursive.discard", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_serviced_recursive.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3058}}},
		{name: "container.cpu.usage.system", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 4525290000000}}},
		{name: "container.cpu.usage.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 8043152341}}},
		{name: "container.cpu.usage.kernelmode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 970000000}}},
		{name: "container.cpu.usage.usermode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 3510000000}}},
		{name: "container.cpu.throttling_data.periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0.19316}}},
		{name: "container.memory.usage.limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 1026359296}}},
		{name: "container.memory.usage.total", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 75915264}}},
		{name: "container.memory.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 7.396558329608582}}},
		{name: "container.memory.usage.max", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 325246976}}},
		{name: "container.memory.active_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.active_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.cache", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.dirty", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.hierarchical_memory_limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 9223372036854771712}}},
		{name: "container.memory.hierarchical_memsw_limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.mapped_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.pgfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.pgmajfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.pgpgin", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.pgpgout", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.rss", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.rss_huge", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_active_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.total_active_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.total_cache", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.total_dirty", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.total_mapped_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.total_pgfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.total_pgmajfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.total_pgpgin", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.total_pgpgout", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.total_rss", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.total_rss_huge", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_unevictable", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_writeback", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.unevictable", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.writeback", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.network.io.usage.rx_bytes", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2787669}}},
		{name: "container.network.io.usage.tx_bytes", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2275281}}},
		{name: "container.network.io.usage.rx_dropped", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_errors", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_packets", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 16598}}},
		{name: "container.network.io.usage.tx_dropped", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_errors", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_packets", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 9050}}},
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
	expected []Metric,
	labels map[string]string,
	actual *consumerdata.MetricsData,
) {
	// Timestamps are generated per ContainerStatsToMetrics call so should be conserved
	ts := actual.Metrics[0].Timeseries[0].Points[0].Timestamp
	expectedMd := metricsData(ts, labels, expected...)

	// Separate for debuggability
	assert.Equal(t, expectedMd.Node, actual.Node)
	assert.Equal(t, expectedMd.Resource, actual.Resource)
	assert.Equal(t, expectedMd.Metrics, actual.Metrics)
	// To fully confirm
	assert.Equal(t, expectedMd, actual)
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

	md, err := ContainerStatsToMetrics(stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	metrics := []Metric{
		{name: "container.cpu.usage.system", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.kernelmode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.usage.usermode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0}}},
		{name: "container.memory.usage.limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.usage.total", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0}}},
		{name: "container.memory.usage.max", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
	}
	assertMetricsDataEqual(t, metrics, nil, md)
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

	md, err := ContainerStatsToMetrics(stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	assertMetricsDataEqual(t, defaultMetrics(), nil, md)
}

func TestStatsToAllMetrics(t *testing.T) {
	stats := statsJSON(t)
	containers := containerJSON(t)
	config := &Config{
		ProvidePerCoreCPUMetrics: true,
	}

	md, err := ContainerStatsToMetrics(stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	metrics := []Metric{
		{name: "container.blockio.io_service_bytes_recursive.read", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 56500224}}},
		{name: "container.blockio.io_service_bytes_recursive.write", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 12103680}}},
		{name: "container.blockio.io_service_bytes_recursive.sync", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 65314816}}},
		{name: "container.blockio.io_service_bytes_recursive.async", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3289088}}},
		{name: "container.blockio.io_service_bytes_recursive.discard", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_service_bytes_recursive.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 68603904}}},
		{name: "container.blockio.io_serviced_recursive.read", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 985}}},
		{name: "container.blockio.io_serviced_recursive.write", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2073}}},
		{name: "container.blockio.io_serviced_recursive.sync", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 2902}}},
		{name: "container.blockio.io_serviced_recursive.async", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 156}}},
		{name: "container.blockio.io_serviced_recursive.discard", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 0}}},
		{name: "container.blockio.io_serviced_recursive.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"device_major", "device_minor"}, values: []Value{{labelValues: []string{"202", "0"}, value: 3058}}},
		{name: "container.cpu.usage.system", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 4525290000000}}},
		{name: "container.cpu.usage.total", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 8043152341}}},
		{name: "container.cpu.usage.kernelmode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 970000000}}},
		{name: "container.cpu.usage.usermode", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 3510000000}}},
		{name: "container.cpu.throttling_data.periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_periods", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.throttling_data.throttled_time", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "ns", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.cpu.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 0.19316}}},
		{
			name:      "container.cpu.usage.percpu",
			mtype:     metricspb.MetricDescriptor_CUMULATIVE_INT64,
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
		{name: "container.memory.usage.limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 1026359296}}},
		{name: "container.memory.usage.total", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 75915264}}},
		{name: "container.memory.percent", mtype: metricspb.MetricDescriptor_GAUGE_DOUBLE, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, doubleValue: 7.396558329608582}}},
		{name: "container.memory.usage.max", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 325246976}}},
		{name: "container.memory.active_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.active_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.cache", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.dirty", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.hierarchical_memory_limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 9223372036854771712}}},
		{name: "container.memory.hierarchical_memsw_limit", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.inactive_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.mapped_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.pgfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.pgmajfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.pgpgin", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.pgpgout", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.rss", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.rss_huge", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_active_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72585216}}},
		{name: "container.memory.total_active_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40316928}}},
		{name: "container.memory.total_cache", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 80760832}}},
		{name: "container.memory.total_dirty", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_anon", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_inactive_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 40579072}}},
		{name: "container.memory.total_mapped_file", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 37711872}}},
		{name: "container.memory.total_pgfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 21714}}},
		{name: "container.memory.total_pgmajfault", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 396}}},
		{name: "container.memory.total_pgpgin", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 85140}}},
		{name: "container.memory.total_pgpgout", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: nil, values: []Value{{labelValues: nil, value: 47694}}},
		{name: "container.memory.total_rss", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 72568832}}},
		{name: "container.memory.total_rss_huge", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_unevictable", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.total_writeback", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.unevictable", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.memory.writeback", mtype: metricspb.MetricDescriptor_GAUGE_INT64, unit: "By", labelKeys: nil, values: []Value{{labelValues: nil, value: 0}}},
		{name: "container.network.io.usage.rx_bytes", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2787669}}},
		{name: "container.network.io.usage.tx_bytes", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "By", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 2275281}}},
		{name: "container.network.io.usage.rx_dropped", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_errors", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.rx_packets", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 16598}}},
		{name: "container.network.io.usage.tx_dropped", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_errors", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 0}}},
		{name: "container.network.io.usage.tx_packets", mtype: metricspb.MetricDescriptor_CUMULATIVE_INT64, unit: "1", labelKeys: []string{"interface"}, values: []Value{{labelValues: []string{"eth0"}, value: 9050}}},
	}

	assertMetricsDataEqual(t, metrics, nil, md)
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

	md, err := ContainerStatsToMetrics(stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	expectedLabels := map[string]string{
		"my.env.to.metric.label":       "my_env_var_value",
		"my.other.env.to.metric.label": "my_other_env_var_value",
	}

	assertMetricsDataEqual(t, defaultMetrics(), expectedLabels, md)
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

	md, err := ContainerStatsToMetrics(stats, containers, config)
	assert.Nil(t, err)
	assert.NotNil(t, md)

	expectedLabels := map[string]string{
		"my.docker.to.metric.label":       "my_specified_docker_label_value",
		"my.other.docker.to.metric.label": "other_specified_docker_label_value",
	}

	assertMetricsDataEqual(t, defaultMetrics(), expectedLabels, md)
}
