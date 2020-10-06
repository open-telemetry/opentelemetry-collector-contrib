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

// Package elastic contains an opentelemetry-collector exporter
// for Elastic APM.
package elastic

import (
	"sort"
	"strings"
	"time"

	"go.elastic.co/apm/model"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/consumer/pdata"
)

// EncodeMetrics encodes an OpenTelemetry metrics slice, and instrumentation
// library information, as one or more metricset lines, writing to w.
//
// TODO(axw) otlpLibrary is currently not used. We should consider recording
// it as metadata.
func EncodeMetrics(otlpMetrics pdata.MetricSlice, otlpLibrary pdata.InstrumentationLibrary, w *fastjson.Writer) (dropped int, _ error) {
	var metricsets metricsets
	for i := 0; i < otlpMetrics.Len(); i++ {
		metric := otlpMetrics.At(i)
		if metric.IsNil() {
			dropped++
			continue
		}

		name := metric.Name()
		switch metric.DataType() {
		case pdata.MetricDataTypeIntGauge:
			intGauge := metric.IntGauge()
			if intGauge.IsNil() {
				dropped++
				continue
			}
			dps := intGauge.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				if dp.IsNil() {
					dropped++
					continue
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.LabelsMap()),
					Samples: map[string]model.Metric{name: {
						Value: float64(dp.Value()),
					}},
				})
			}
		case pdata.MetricDataTypeDoubleGauge:
			doubleGauge := metric.DoubleGauge()
			if doubleGauge.IsNil() {
				dropped++
				continue
			}
			dps := doubleGauge.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				if dp.IsNil() {
					dropped++
					continue
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.LabelsMap()),
					Samples: map[string]model.Metric{name: {
						Value: dp.Value(),
					}},
				})
			}
		case pdata.MetricDataTypeIntSum:
			intSum := metric.IntSum()
			if intSum.IsNil() {
				dropped++
				continue
			}
			dps := intSum.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				if dp.IsNil() {
					dropped++
					continue
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.LabelsMap()),
					Samples: map[string]model.Metric{name: {
						Value: float64(dp.Value()),
					}},
				})
			}
		case pdata.MetricDataTypeDoubleSum:
			doubleSum := metric.DoubleSum()
			if doubleSum.IsNil() {
				dropped++
				continue
			}
			dps := doubleSum.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				if dp.IsNil() {
					dropped++
					continue
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.LabelsMap()),
					Samples: map[string]model.Metric{name: {
						Value: dp.Value(),
					}},
				})
			}
		case pdata.MetricDataTypeIntHistogram:
			// TODO(axw) requires https://github.com/elastic/apm-server/issues/3195
			intHistogram := metric.IntHistogram()
			if intHistogram.IsNil() {
				dropped++
				continue
			}
			dropped += intHistogram.DataPoints().Len()
		case pdata.MetricDataTypeDoubleHistogram:
			// TODO(axw) requires https://github.com/elastic/apm-server/issues/3195
			doubleHistogram := metric.DoubleHistogram()
			if doubleHistogram.IsNil() {
				dropped++
				continue
			}
			dropped += doubleHistogram.DataPoints().Len()
		default:
			// Unknown type, so just increment dropped by 1 as a best effort.
			dropped++
		}
	}
	for _, metricset := range metricsets {
		w.RawString(`{"metricset":`)
		if err := metricset.MarshalFastJSON(w); err != nil {
			return dropped, err
		}
		w.RawString("}\n")
	}
	return dropped, nil
}

func asTime(in pdata.TimestampUnixNano) model.Time {
	return model.Time(time.Unix(0, int64(in)))
}

func asStringMap(in pdata.StringMap) model.StringMap {
	var out model.StringMap
	in.Sort()
	in.ForEach(func(k string, v pdata.StringValue) {
		out = append(out, model.StringMapItem{
			Key:   k,
			Value: v.Value(),
		})
	})
	return out
}

type metricsets []model.Metrics

func (ms *metricsets) upsert(m model.Metrics) {
	i := ms.search(m)
	if i < len(*ms) && compareMetricsets((*ms)[i], m) == 0 {
		existing := (*ms)[i]
		for k, v := range m.Samples {
			existing.Samples[k] = v
		}
	} else {
		head := (*ms)[:i]
		tail := append([]model.Metrics{m}, (*ms)[i:]...)
		*ms = append(head, tail...)
	}
}

func (ms *metricsets) search(m model.Metrics) int {
	return sort.Search(len(*ms), func(i int) bool {
		return compareMetricsets((*ms)[i], m) >= 0
	})
}

func compareMetricsets(a, b model.Metrics) int {
	atime, btime := time.Time(a.Timestamp), time.Time(b.Timestamp)
	if atime.Before(btime) {
		return -1
	} else if atime.After(btime) {
		return 1
	}
	n := len(a.Labels) - len(b.Labels)
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	for i, la := range a.Labels {
		lb := b.Labels[i]
		if n := strings.Compare(la.Key, lb.Key); n != 0 {
			return n
		}
		if n := strings.Compare(la.Value, lb.Value); n != 0 {
			return n
		}
	}
	return 0
}
