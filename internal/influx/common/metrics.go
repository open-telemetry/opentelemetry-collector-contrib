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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"

type InfluxMetricValueType uint8

const (
	InfluxMetricValueTypeUntyped InfluxMetricValueType = iota
	InfluxMetricValueTypeGauge
	InfluxMetricValueTypeSum
	InfluxMetricValueTypeHistogram
	InfluxMetricValueTypeSummary
)

func (vType InfluxMetricValueType) String() string {
	switch vType {
	case InfluxMetricValueTypeUntyped:
		return "untyped"
	case InfluxMetricValueTypeGauge:
		return "gauge"
	case InfluxMetricValueTypeSum:
		return "sum"
	case InfluxMetricValueTypeHistogram:
		return "histogram"
	case InfluxMetricValueTypeSummary:
		return "summary"
	default:
		panic("invalid InfluxMetricValueType")
	}
}

type MetricsSchema uint8

const (
	_ MetricsSchema = iota
	MetricsSchemaTelegrafPrometheusV1
	MetricsSchemaTelegrafPrometheusV2
	MetricsSchemaOtelV1
)

func (ms MetricsSchema) String() string {
	switch ms {
	case MetricsSchemaTelegrafPrometheusV1:
		return "telegraf-prometheus-v1"
	case MetricsSchemaTelegrafPrometheusV2:
		return "telegraf-prometheus-v2"
	case MetricsSchemaOtelV1:
		return "otel-v1"
	default:
		panic("invalid MetricsSchema")
	}
}

var MetricsSchemata = map[string]MetricsSchema{
	MetricsSchemaTelegrafPrometheusV1.String(): MetricsSchemaTelegrafPrometheusV1,
	MetricsSchemaTelegrafPrometheusV2.String(): MetricsSchemaTelegrafPrometheusV2,
	MetricsSchemaOtelV1.String():               MetricsSchemaOtelV1,
}
