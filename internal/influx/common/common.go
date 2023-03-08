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

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
)

// https://github.com/open-telemetry/opentelemetry-specification/tree/v1.16.0/specification/resource/semantic_conventions
var ResourceNamespace = regexp.MustCompile(generateResourceNamespaceRegexp())

func generateResourceNamespaceRegexp() string {
	semconvResourceAttributeNames := semconv.GetResourceSemanticConventionAttributeNames()
	components := make([]string, len(semconvResourceAttributeNames))
	for i, attributeName := range semconvResourceAttributeNames {
		components[i] = strings.ReplaceAll(attributeName, `.`, `\.`)
	}
	return `^(?:` + strings.Join(components, `|`) + `)(?:\.[a-z0-9]+)*$`
}

const (
	MeasurementSpans      = "spans"
	MeasurementSpanLinks  = "span-links"
	MeasurementLogs       = "logs"
	MeasurementPrometheus = "prometheus"

	MetricGaugeFieldKey          = "gauge"
	MetricCounterFieldKey        = "counter"
	MetricHistogramCountFieldKey = "count"
	MetricHistogramSumFieldKey   = "sum"
	MetricHistogramInfFieldKey   = "inf"
	MetricHistogramBoundKeyV2    = "le"
	MetricHistogramCountSuffix   = "_count"
	MetricHistogramSumSuffix     = "_sum"
	MetricHistogramBucketSuffix  = "_bucket"
	MetricSummaryCountFieldKey   = "count"
	MetricSummarySumFieldKey     = "sum"
	MetricSummaryQuantileKeyV2   = "quantile"
	MetricSummaryCountSuffix     = "_count"
	MetricSummarySumSuffix       = "_sum"

	// These attribute key names are influenced by the proto message keys.
	// https://github.com/open-telemetry/opentelemetry-proto/blob/abbf7b7b49a5342d0d6c0e86e91d713bbedb6580/opentelemetry/proto/trace/v1/trace.proto
	// https://github.com/open-telemetry/opentelemetry-proto/blob/abbf7b7b49a5342d0d6c0e86e91d713bbedb6580/opentelemetry/proto/metrics/v1/metrics.proto
	// https://github.com/open-telemetry/opentelemetry-proto/blob/abbf7b7b49a5342d0d6c0e86e91d713bbedb6580/opentelemetry/proto/logs/v1/logs.proto
	AttributeTime                   = "time"
	AttributeStartTimeUnixNano      = "start_time_unix_nano"
	AttributeTraceID                = "trace_id"
	AttributeSpanID                 = "span_id"
	AttributeTraceState             = "trace_state"
	AttributeParentSpanID           = "parent_span_id"
	AttributeParentServiceName      = "parent_service_name"
	AttributeChildServiceName       = "child_service_name"
	AttributeCallCount              = "call_count"
	AttributeSpansQueueDepth        = "spans_queue_depth"
	AttributeSpansDropped           = "spans_dropped"
	AttributeName                   = "name"
	AttributeSpanKind               = "kind"
	AttributeEndTimeUnixNano        = "end_time_unix_nano"
	AttributeDurationNano           = "duration_nano"
	AttributeDroppedAttributesCount = "dropped_attributes_count"
	AttributeDroppedEventsCount     = "dropped_events_count"
	AttributeDroppedLinksCount      = "dropped_links_count"
	AttributeAttributes             = "attributes"
	AttributeLinkedTraceID          = "linked_trace_id"
	AttributeLinkedSpanID           = "linked_span_id"
	AttributeSeverityNumber         = "severity_number"
	AttributeSeverityText           = "severity_text"
	AttributeBody                   = "body"
)

func AttributeValueToInfluxTagValue(value pcommon.Value) (string, error) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str(), nil
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(value.Int(), 10), nil
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(value.Double(), 'f', -1, 64), nil
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(value.Bool()), nil
	case pcommon.ValueTypeMap:
		jsonBytes, err := json.Marshal(otlpKeyValueListToMap(value.Map()))
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeSlice:
		jsonBytes, err := json.Marshal(otlpArrayToSlice(value.Slice()))
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeEmpty:
		return "", nil
	default:
		return "", fmt.Errorf("unknown value type %d", value.Type())
	}
}

func otlpKeyValueListToMap(kvList pcommon.Map) map[string]interface{} {
	m := make(map[string]interface{}, kvList.Len())
	kvList.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			m[k] = v.Str()
		case pcommon.ValueTypeInt:
			m[k] = v.Int()
		case pcommon.ValueTypeDouble:
			m[k] = v.Double()
		case pcommon.ValueTypeBool:
			m[k] = v.Bool()
		case pcommon.ValueTypeMap:
			m[k] = otlpKeyValueListToMap(v.Map())
		case pcommon.ValueTypeSlice:
			m[k] = otlpArrayToSlice(v.Slice())
		case pcommon.ValueTypeEmpty:
			m[k] = nil
		default:
			m[k] = fmt.Sprintf("<invalid map value> %v", v)
		}
		return true
	})
	return m
}

func otlpArrayToSlice(arr pcommon.Slice) []interface{} {
	s := make([]interface{}, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		v := arr.At(i)
		switch v.Type() {
		case pcommon.ValueTypeStr:
			s = append(s, v.Str())
		case pcommon.ValueTypeInt:
			s = append(s, v.Int())
		case pcommon.ValueTypeDouble:
			s = append(s, v.Double())
		case pcommon.ValueTypeBool:
			s = append(s, v.Bool())
		case pcommon.ValueTypeEmpty:
			s = append(s, nil)
		default:
			s = append(s, fmt.Sprintf("<invalid array value> %v", v))
		}
	}
	return s
}
