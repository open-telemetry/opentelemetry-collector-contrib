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
)

func TestMetricValue(t *testing.T) {
	var (
		name  string   = "name"
		value float64  = math.Pi
		ts    uint64   = uint64(time.Now().UnixNano())
		tags  []string = []string{"tool:opentelemetry", "version:0.1.0"}
	)

	metric := newGauge(name, ts, value, tags)
	assert.Equal(t, Gauge, metric.GetType())
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

	point := pdata.NewIntDataPoint()
	point.InitEmpty()
	point.SetValue(17)
	point.SetTimestamp(pdata.TimestampUnixNano(ts))
	slice.Append(point)

	nilPoint := pdata.NewIntDataPoint()
	slice.Append(nilPoint)

	assert.ElementsMatch(t,
		mapIntMetrics("int64.test", slice),
		[]datadog.Metric{newGauge("int64.test", uint64(ts), 17, []string{})},
	)
}

func TestMapDoubleMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewDoubleDataPointSlice()

	point := pdata.NewDoubleDataPoint()
	point.InitEmpty()
	point.SetValue(math.Pi)
	point.SetTimestamp(pdata.TimestampUnixNano(ts))
	slice.Append(point)

	nilPoint := pdata.NewDoubleDataPoint()
	slice.Append(nilPoint)

	assert.ElementsMatch(t,
		mapDoubleMetrics("float64.test", slice),
		[]datadog.Metric{newGauge("float64.test", uint64(ts), math.Pi, []string{})},
	)
}

func TestMapIntHistogramMetrics(t *testing.T) {
	ts := time.Now().UnixNano()
	slice := pdata.NewIntHistogramDataPointSlice()

	point := pdata.NewIntHistogramDataPoint()
	point.InitEmpty()
	point.SetCount(20)
	point.SetSum(200)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetTimestamp(pdata.TimestampUnixNano(ts))
	slice.Append(point)

	nilPoint := pdata.NewIntHistogramDataPoint()
	slice.Append(nilPoint)

	noBuckets := []datadog.Metric{
		newGauge("intHist.test.count", uint64(ts), 20, []string{}),
		newGauge("intHist.test.sum", uint64(ts), 200, []string{}),
	}

	buckets := []datadog.Metric{
		newGauge("intHist.test.count_per_bucket", uint64(ts), 2, []string{"bucket_idx:0"}),
		newGauge("intHist.test.count_per_bucket", uint64(ts), 18, []string{"bucket_idx:1"}),
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

	point := pdata.NewDoubleHistogramDataPoint()
	point.InitEmpty()
	point.SetCount(20)
	point.SetSum(math.Pi)
	point.SetBucketCounts([]uint64{2, 18})
	point.SetTimestamp(pdata.TimestampUnixNano(ts))
	slice.Append(point)

	nilPoint := pdata.NewDoubleHistogramDataPoint()
	slice.Append(nilPoint)

	noBuckets := []datadog.Metric{
		newGauge("doubleHist.test.count", uint64(ts), 20, []string{}),
		newGauge("doubleHist.test.sum", uint64(ts), math.Pi, []string{}),
	}

	buckets := []datadog.Metric{
		newGauge("doubleHist.test.count_per_bucket", uint64(ts), 2, []string{"bucket_idx:0"}),
		newGauge("doubleHist.test.count_per_bucket", uint64(ts), 18, []string{"bucket_idx:1"}),
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
