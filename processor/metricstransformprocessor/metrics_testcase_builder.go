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

package metricstransformprocessor

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// Builder is the builder for testcases
type Builder struct {
	testcase *metricspb.Metric
}

// TestcaseBuilder is used to build metrics testcases
func TestcaseBuilder() Builder {
	return Builder{
		testcase: &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{},
		},
	}
}

// SetName sets the name for the testcase
func (b Builder) SetName(name string) Builder {
	b.testcase.MetricDescriptor.Name = name
	return b
}

// SetLabels sets the labels for the testcase
func (b Builder) SetLabels(labels []string) Builder {
	labelKeys := make([]*metricspb.LabelKey, len(labels))
	for i, l := range labels {
		labelKeys[i] = &metricspb.LabelKey{
			Key: l,
		}
	}
	b.testcase.MetricDescriptor.LabelKeys = labelKeys
	return b
}

// SetLabelValues sets the labels values for the timeseries
func (b Builder) SetLabelValues(values [][]string) Builder {
	for i, valueSet := range values {
		labelValues := make([]*metricspb.LabelValue, len(valueSet))
		for j, v := range valueSet {
			labelValues[j] = &metricspb.LabelValue{
				Value: v,
			}
		}
		b.testcase.Timeseries[i].LabelValues = labelValues
	}
	return b
}

// InitTimeseries inits a timeseries that is certain length long
func (b Builder) InitTimeseries(len int) Builder {
	timeseries := make([]*metricspb.TimeSeries, len)
	for i := range timeseries {
		timeseries[i] = &metricspb.TimeSeries{}
	}
	b.testcase.Timeseries = timeseries
	return b
}

// Build builds from the builder to the final metric
func (b Builder) Build() *metricspb.Metric {
	return b.testcase
}
