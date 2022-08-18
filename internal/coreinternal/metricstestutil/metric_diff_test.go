// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricstestutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
)

func TestSameMetrics(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	actual := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	diffs := diffMetricData(expected, actual)
	assert.Nil(t, diffs)
}

func TestDifferentValues(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentNumPts(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.NumPtsPerMetric = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentPtValueTypes(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	cfg2.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 3)
}

func TestAttributes(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	cfg1.NumPtLabels = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	cfg2.NumPtLabels = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 1)
}

func TestExponentialHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	cfg1.PtVal = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricDataTypeHistogram
	cfg2.PtVal = 3
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 3)
}
