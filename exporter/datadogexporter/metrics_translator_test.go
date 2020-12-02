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

package datadogexporter

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metrics"
)

func TestMetricValue(t *testing.T) {
	var (
		name  string   = "name"
		value float64  = math.Pi
		ts    uint64   = uint64(time.Now().UnixNano())
		tags  []string = []string{"tool:opentelemetry", "version:0.1.0"}
	)

	metric := metrics.NewGauge(name, ts, value, tags)
	assert.Equal(t, metrics.Gauge, metric.GetType())
	assert.Equal(t, tags, metric.Tags)
}

func TestGetTags(t *testing.T) {
	labels := pdata.NewStringMap()
	labels.InitFromMap(map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "",
	})

	assert.ElementsMatch(t,
		getTags(labels),
		[...]string{"key1:val1", "key2:val2", "key3:n/a"},
	)
}

func TestMapIntMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewIntDataPointSlice()
	slice.Resize(1)
	point := slice.At(0)
	point.SetValue(17)
	point.SetTimestamp(pdata.TimestampUnixNano(ts))

	assert.ElementsMatch(t,
		mapIntMetrics("int64.test", slice),
		[]datadog.Metric{metrics.NewGauge("int64.test", uint64(ts), 17, []string{})},
	)
}

func TestMapDoubleMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewDoubleDataPointSlice()
	slice.Resize(1)
	point := slice.At(0)
	point.SetValue(math.Pi)
	point.SetTimestamp(pdata.TimestampUnixNano(ts))

	assert.ElementsMatch(t,
		mapDoubleMetrics("float64.test", slice),
		[]datadog.Metric{metrics.NewGauge("float64.test", uint64(ts), math.Pi, []string{})},
	)
}

func TestMapIntHistogramMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewIntHistogramDataPointSlice()
	slice.Resize(1)
	point := slice.At(0)
	point.SetCount(20)
	point.SetSum(200)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetTimestamp(pdata.TimestampUnixNano(ts))

	noBuckets := []datadog.Metric{
		metrics.NewGauge("intHist.test.count", uint64(ts), 20, []string{}),
		metrics.NewGauge("intHist.test.sum", uint64(ts), 200, []string{}),
	}

	buckets := []datadog.Metric{
		metrics.NewGauge("intHist.test.count_per_bucket", uint64(ts), 2, []string{"bucket_idx:0"}),
		metrics.NewGauge("intHist.test.count_per_bucket", uint64(ts), 18, []string{"bucket_idx:1"}),
	}

	assert.ElementsMatch(t,
		mapIntHistogramMetrics("intHist.test", slice, false), // No buckets
		noBuckets,
	)

	assert.ElementsMatch(t,
		mapIntHistogramMetrics("intHist.test", slice, true), // buckets
		append(noBuckets, buckets...),
	)
}

func TestMapDoubleHistogramMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewDoubleHistogramDataPointSlice()
	slice.Resize(1)
	point := slice.At(0)
	point.SetCount(20)
	point.SetSum(math.Pi)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetTimestamp(pdata.TimestampUnixNano(ts))

	noBuckets := []datadog.Metric{
		metrics.NewGauge("doubleHist.test.count", uint64(ts), 20, []string{}),
		metrics.NewGauge("doubleHist.test.sum", uint64(ts), math.Pi, []string{}),
	}

	buckets := []datadog.Metric{
		metrics.NewGauge("doubleHist.test.count_per_bucket", uint64(ts), 2, []string{"bucket_idx:0"}),
		metrics.NewGauge("doubleHist.test.count_per_bucket", uint64(ts), 18, []string{"bucket_idx:1"}),
	}

	assert.ElementsMatch(t,
		mapDoubleHistogramMetrics("doubleHist.test", slice, false), // No buckets
		noBuckets,
	)

	assert.ElementsMatch(t,
		mapDoubleHistogramMetrics("doubleHist.test", slice, true), // buckets
		append(noBuckets, buckets...),
	)
}
