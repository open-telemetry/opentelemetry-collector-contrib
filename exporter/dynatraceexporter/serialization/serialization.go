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

package serialization

import (
	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/model/pdata"
)

func getDimsFromMetric(attributes pdata.AttributeMap) dimensions.NormalizedDimensionList {
	dimsFromMetric := []dimensions.Dimension{}
	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		dimsFromMetric = append(dimsFromMetric, dimensions.NewDimension(k, v.AsString()))
		return true
	})
	return dimensions.NewNormalizedDimensionList(dimsFromMetric...)
}

// SerializeNumberDataPoints serializes a slice of number datapoints to a Dynatrace gauge.
func SerializeNumberDataPoints(prefix, name string, data pdata.NumberDataPointSlice, defaultDimensions dimensions.NormalizedDimensionList) []string {
	// {name} {value} {timestamp}
	output := []string{}
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)

		var val dtMetric.MetricOption
		switch p.Type() {
		case pdata.MetricValueTypeDouble:
			val = dtMetric.WithFloatGaugeValue(p.DoubleVal())
		case pdata.MetricValueTypeInt:
			val = dtMetric.WithIntGaugeValue(p.IntVal())
		}

		dimsFromMetric := getDimsFromMetric(p.Attributes())

		m, err := dtMetric.NewMetric(
			name,
			dtMetric.WithPrefix(prefix),
			dtMetric.WithDimensions(
				dimensions.MergeLists(
					defaultDimensions,
					dimsFromMetric,
				),
			),
			dtMetric.WithTimestamp(p.Timestamp().AsTime()),
			val,
		)

		if err != nil {
			continue
		}

		serialized, err := m.Serialize()

		if err != nil {
			continue
		}

		output = append(output, serialized)
	}

	return output
}

// SerializeHistogramMetrics serializes a slice of double histogram datapoints to a Dynatrace gauge.
//
// IMPORTANT: Min and max are required by Dynatrace but not provided by histogram so they are assumed to be the average.
func SerializeHistogramMetrics(prefix, name string, data pdata.HistogramDataPointSlice, defaultDimensions dimensions.NormalizedDimensionList) []string {
	// {name} gauge,min=9.75,max=9.75,sum=19.5,count=2 {timestamp_unix_ms}
	output := []string{}
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		if p.Count() == 0 {
			continue
		}
		avg := p.Sum() / float64(p.Count())

		dimsFromMetric := getDimsFromMetric(p.Attributes())
		m, err := dtMetric.NewMetric(
			name,
			dtMetric.WithPrefix(prefix),
			dtMetric.WithDimensions(dimensions.MergeLists(defaultDimensions, dimsFromMetric)),
			dtMetric.WithTimestamp(p.Timestamp().AsTime()),
			dtMetric.WithFloatSummaryValue(avg, avg, p.Sum(), int64(p.Count())),
		)

		if err != nil {
			continue
		}

		serialized, err := m.Serialize()

		if err != nil {
			continue
		}

		output = append(output, serialized)
	}

	return output
}
