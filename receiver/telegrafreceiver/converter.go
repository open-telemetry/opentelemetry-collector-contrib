// Copyright 2021, OpenTelemetry Authors
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

package telegrafreceiver

import (
	"fmt"
	"time"

	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	fieldLabel = "field"
)

type MetricConverter interface {
	Convert(telegraf.Metric) (pdata.Metrics, error)
}

type metricConverter struct {
	separateField bool
	logger        *zap.Logger
}

func newConverter(separateField bool, logger *zap.Logger) MetricConverter {
	return metricConverter{
		separateField: separateField,
		logger:        logger,
	}
}

// Convert converts telegraf.Metric to pdata.Metrics.
func (mc metricConverter) Convert(m telegraf.Metric) (pdata.Metrics, error) {
	ms := pdata.NewMetrics()
	rms := ms.ResourceMetrics()
	rms.Resize(1)
	rm := rms.At(0)

	// Attach tags as attributes - pipe through the metadata
	for _, t := range m.TagList() {
		rm.Resource().Attributes().InsertString(t.Key, t.Value)
	}

	rm.InstrumentationLibraryMetrics().Resize(1)
	ilm := rm.InstrumentationLibraryMetrics().At(0)

	il := ilm.InstrumentationLibrary()
	il.SetName(typeStr)
	il.SetVersion(versionStr)

	tim := m.Time()

	metrics := ilm.Metrics()

	switch t := m.Type(); t {
	case telegraf.Gauge:
		for _, f := range m.FieldList() {
			var pm pdata.Metric

			switch v := f.Value.(type) {
			case float64:
				pm = newDoubleGauge(f.Key, v, tim, mc.separateField)

			case int64:
				pm = newIntGauge(f.Key, v, tim, mc.separateField)
			case uint64:
				pm = newIntGauge(f.Key, int64(v), tim, mc.separateField)

			case bool:
				var vv int64 = 0
				if v {
					vv = 1
				}
				pm = newIntGauge(f.Key, vv, tim, mc.separateField)

			default:
				mc.logger.Debug(
					"Unsupported data type when handling telegraf.Gauge",
					zap.String("type", fmt.Sprintf("%T", v)),
					zap.String("key", f.Key),
					zap.Any("value", f.Value),
				)
				continue
			}

			setName(&pm, m.Name(), f.Key, mc.separateField)
			metrics.Append(pm)
		}

	case telegraf.Untyped:
		for _, f := range m.FieldList() {
			var pm pdata.Metric

			switch v := f.Value.(type) {
			case float64:
				pm = newDoubleGauge(f.Key, v, tim, mc.separateField)

			case int64:
				pm = newIntGauge(f.Key, v, tim, mc.separateField)
			case uint64:
				pm = newIntGauge(f.Key, int64(v), tim, mc.separateField)

			case bool:
				var vv int64 = 0
				if v {
					vv = 1
				}
				pm = newIntGauge(f.Key, vv, tim, mc.separateField)

			default:
				mc.logger.Debug(
					"Unsupported data type when handling telegraf.Untyped",
					zap.String("type", fmt.Sprintf("%T", v)),
					zap.String("key", f.Key),
					zap.Any("value", f.Value),
				)
				continue
			}

			setName(&pm, m.Name(), f.Key, mc.separateField)
			metrics.Append(pm)
		}

	case telegraf.Counter:
		return pdata.Metrics{}, fmt.Errorf("unsupported metric type: telegraf.Counter")
	case telegraf.Summary:
		return pdata.Metrics{}, fmt.Errorf("unsupported metric type: telegraf.Summary")
	case telegraf.Histogram:
		return pdata.Metrics{}, fmt.Errorf("unsupported metric type: telegraf.Histogram")

	default:
		return pdata.Metrics{}, fmt.Errorf("unknown metric type: %T", t)
	}

	return ms, nil
}

func setName(pm *pdata.Metric, name string, key string, separateField bool) {
	if separateField {
		pm.SetName(name)
	} else {
		pm.SetName(name + "_" + key)
	}
}

func newDoubleGauge(key string, value float64, t time.Time, separateField bool) pdata.Metric {
	pm := pdata.NewMetric()
	pm.SetDataType(pdata.MetricDataTypeDoubleGauge)
	dps := pm.DoubleGauge().DataPoints()
	dps.Resize(1)
	dp := dps.At(0)
	dp.SetValue(value)
	dp.SetTimestamp(pdata.Timestamp(t.UnixNano()))
	if separateField {
		dp.LabelsMap().Insert(fieldLabel, key)
	}
	return pm
}

func newIntGauge(key string, value int64, t time.Time, separateField bool) pdata.Metric {
	pm := pdata.NewMetric()
	pm.SetDataType(pdata.MetricDataTypeIntGauge)
	dps := pm.IntGauge().DataPoints()
	dps.Resize(1)
	dp := dps.At(0)
	dp.SetValue(value)
	dp.SetTimestamp(pdata.Timestamp(t.UnixNano()))
	if separateField {
		dp.LabelsMap().Insert(fieldLabel, key)
	}
	return pm
}
