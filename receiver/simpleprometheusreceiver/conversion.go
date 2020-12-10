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

package simpleprometheusreceiver

import (
	"fmt"
	"net"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

const (
	portAttr   = "port"
	schemeAttr = "scheme"
)

type extractor interface {
	DataType() pdata.MetricDataType
	AddDataPoint(*dto.Metric, pdata.Metric, map[string]string)
}

type gaugeExtractor struct {
	ts pdata.TimestampUnixNano
}

func (e gaugeExtractor) DataType() pdata.MetricDataType {
	return pdata.MetricDataTypeDoubleGauge
}

func (e gaugeExtractor) AddDataPoint(m *dto.Metric, md pdata.Metric, labels map[string]string) {
	val := m.GetGauge().GetValue()

	dps := md.DoubleGauge().DataPoints()
	dps.Resize(1)
	dp := dps.At(0)
	dp.LabelsMap().InitFromMap(labels)
	dp.SetTimestamp(e.ts)
	dp.SetValue(val)
}

type counterExtractor struct {
	// has startTs included in timeseries
	ts pdata.TimestampUnixNano
}

func (e counterExtractor) DataType() pdata.MetricDataType {
	return pdata.MetricDataTypeDoubleSum
}

func (e counterExtractor) AddDataPoint(m *dto.Metric, md pdata.Metric, labels map[string]string) {
	val := m.GetCounter().GetValue()

	dps := md.DoubleSum().DataPoints()
	dps.Resize(1)
	dp := dps.At(0)
	dp.LabelsMap().InitFromMap(labels)
	dp.SetStartTime(e.ts)
	dp.SetTimestamp(e.ts)
	dp.SetValue(val)
}

type untypedExtractor struct {
	ts pdata.TimestampUnixNano
}

func (e untypedExtractor) DataType() pdata.MetricDataType {
	return pdata.MetricDataTypeDoubleGauge
}

func (e untypedExtractor) AddDataPoint(m *dto.Metric, md pdata.Metric, labels map[string]string) {
	val := m.GetUntyped().GetValue()

	dps := md.DoubleGauge().DataPoints()
	dps.Resize(1)
	dp := dps.At(0)
	dp.LabelsMap().InitFromMap(labels)
	dp.SetTimestamp(e.ts)
	dp.SetValue(val)
}

func fillResourceMetrics(rm pdata.ResourceMetrics, labels map[string]string) {
	job, instance := labels[model.JobLabel], labels[model.InstanceLabel]
	host, port, err := net.SplitHostPort(instance)
	if err != nil {
		host = instance
	}

	rm.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		conventions.AttributeServiceName: pdata.NewAttributeValueString(job),
		conventions.AttributeHostName:    pdata.NewAttributeValueString(host),
		portAttr:                         pdata.NewAttributeValueString(port),
		// TODO: Figure out how to retrieve scheme
		// schemeAttr: pdata.NewAttributeValueString(scheme),
	})
}

func processMetricFamily(mf *dto.MetricFamily, rms pdata.ResourceMetricsSlice, ext extractor, config *Config) {
	metricName := mf.GetName()
	description := mf.GetHelp()
	dataType := ext.DataType()

	oldLen := rms.Len()
	rms.Resize(oldLen + len(mf.GetMetric()))

	idx := 0
	for _, m := range mf.GetMetric() {
		if m == nil {
			continue
		}

		labels := labelPairsToLabelsMap(m.GetLabel(), config)

		rm := rms.At(idx)
		fillResourceMetrics(rm, labels)
		ilms := rm.InstrumentationLibraryMetrics()
		ilms.Resize(1)
		metrics := ilms.At(0).Metrics()
		metrics.Resize(1)
		md := metrics.At(0)
		md.SetName(metricName)
		md.SetDescription(description)
		md.SetDataType(dataType)
		ext.AddDataPoint(m, md, labels)

		idx++
	}

	rms.Resize(oldLen + idx)
}

func labelPairsToLabelsMap(pairs []*dto.LabelPair, config *Config) map[string]string {
	labels := make(map[string]string, len(pairs) + 2)
	// Add custom labels
	labels[model.JobLabel] = fmt.Sprintf("%s/%s", typeStr, config.Endpoint)
	labels[model.InstanceLabel] = config.Endpoint
	for _, p := range pairs {
		if p == nil {
			continue
		}
		labels[p.GetName()] = p.GetValue()
	}
	return labels
}

func convertMetricFamily(mf *dto.MetricFamily, rms pdata.ResourceMetricsSlice, ts int64, config *Config) {
	if mf.Type == nil || mf.Name == nil {
		return
	}

	timestamp := pdata.TimestampUnixNano(ts)

	var ext extractor
	switch *mf.Type {
	case dto.MetricType_GAUGE:
		ext = gaugeExtractor{ts: timestamp}
	case dto.MetricType_COUNTER:
		ext = counterExtractor{ts: timestamp}
	case dto.MetricType_UNTYPED:
		ext = untypedExtractor{ts: timestamp}
	default:
		return
	}

	processMetricFamily(mf, rms, ext, config)
}
