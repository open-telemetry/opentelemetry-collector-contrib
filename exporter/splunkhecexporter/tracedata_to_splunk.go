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

package splunkhecexporter

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/openzipkin/zipkin-go/model"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

type splunkEvent struct {
	Time       float64     `json:"time"`                 // epoch time
	Host       string      `json:"host"`                 // hostname
	Source     string      `json:"source,omitempty"`     // optional description of the source of the event; typically the app's name
	SourceType string      `json:"sourcetype,omitempty"` // optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	Index      string      `json:"index,omitempty"`      // optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Event      interface{} `json:"event"`                // Payload of the event.
}

func traceDataToSplunk(logger *zap.Logger, data consumerdata.TraceData, config *Config) ([]*splunkEvent, int) {
	var host string
	if data.Resource != nil {
		host = data.Resource.Labels[hostnameLabel]
	}
	if host == "" {
		host = unknownHostName
	}
	numDroppedSpans := 0
	splunkEvents := make([]*splunkEvent, 0)
	for _, span := range data.Spans {
		if span.StartTime == nil {
			logger.Debug(
				"Span dropped as it had no start timestamp",
				zap.Any("span", span))
			numDroppedSpans++
			continue
		}
		zipkinSpan, err := convertToZipkinSpan(span)
		if err != nil {
			logger.Debug(
				"Span dropped as it could not be converted to zipkin format",
				zap.Any("span", span), zap.Error(err))
			numDroppedSpans++
			continue
		}

		se := &splunkEvent{
			Time:       timestampToEpochMilliseconds(span.StartTime),
			Host:       host,
			Source:     config.Source,
			SourceType: config.SourceType,
			Index:      config.Index,
			Event:      zipkinSpan,
		}
		splunkEvents = append(splunkEvents, se)
	}

	return splunkEvents, numDroppedSpans
}

func convertKind(kind tracepb.Span_SpanKind) model.Kind {
	switch kind {
	case tracepb.Span_SERVER:
		return model.Server
	case tracepb.Span_CLIENT:
		return model.Client
	// TODO newer versions of the opencensus model supports those additional fields.
	//case tracepb.Span_CONSUMER:
	//	return model.Consumer
	//case tracepb.Span_PRODUCER:
	//	return model.Producer
	//case tracepb.Span_INTERNAL:
	case tracepb.Span_SPAN_KIND_UNSPECIFIED:
	default:
		return model.Undetermined
	}
	return model.Undetermined
}

func uint64frombytes(bytes []byte) uint64 {
	if len(bytes) == 0 {
		return 0
	}
	bits := binary.LittleEndian.Uint64(bytes)
	return bits
}

func convertToZipkinSpan(span *tracepb.Span) (model.SpanModel, error) {
	spanID := model.ID(uint64frombytes(span.GetSpanId()))
	parentID := model.ID(uint64frombytes(span.GetParentSpanId()))
	startTime := time.Unix(span.GetStartTime().GetSeconds(), int64(span.GetStartTime().GetNanos()))
	endTime := time.Unix(span.GetEndTime().GetSeconds(), int64(span.GetEndTime().GetNanos()))
	tags, err := convertTags(span.GetAttributes())
	if err != nil {
		return model.SpanModel{}, err
	}
	tags["ot.status_code"] = string(span.GetStatus().Code)
	if span.GetStatus().GetMessage() != "" {
		tags["ot.status_description"] = span.GetStatus().GetMessage()
	}
	annotations := convertAnnotations(span.GetTimeEvents())
	zipkinSpan := model.SpanModel{
		SpanContext: model.SpanContext{
			TraceID: model.TraceID{
				High: uint64frombytes(span.GetTraceId()[0:8]),
				Low:  uint64frombytes(span.GetTraceId()[8:16]),
			},
			ID:       spanID,
			ParentID: &parentID,
			Debug:    false,
			Sampled:  nil,
			Err:      nil,
		},
		Name:           span.GetName().GetValue(),
		Kind:           convertKind(span.GetKind()),
		Timestamp:      startTime,
		Duration:       endTime.Sub(startTime),
		Shared:         false,
		LocalEndpoint:  nil,
		RemoteEndpoint: nil,
		Annotations:    annotations,
		Tags:           tags,
	}
	return zipkinSpan, nil
}

func convertAnnotations(events *tracepb.Span_TimeEvents) []model.Annotation {
	var annotations []model.Annotation

	for _, event := range events.GetTimeEvent() {
		var value string
		if event.GetMessageEvent() != nil {
			value = fmt.Sprintf("%d", event.GetMessageEvent().GetId())
		} else {
			value = event.GetAnnotation().GetDescription().GetValue()
		}

		// TODO the specification asks to map key/value pairs, but the annotation model just has a Value field.
		annotation := model.Annotation{
			Timestamp: time.Unix(event.GetTime().GetSeconds(), int64(event.GetTime().GetNanos())),
			Value:     value,
		}
		annotations = append(annotations, annotation)
	}
	return annotations
}

func convertValue(attrValue *tracepb.AttributeValue) (string, error) {
	if attrValue.GetStringValue() != nil {
		return attrValue.GetStringValue().GetValue(), nil
	} else if _, ok := attrValue.GetValue().(*tracepb.AttributeValue_DoubleValue); ok {
		return fmt.Sprintf("%f", attrValue.GetDoubleValue()), nil
	} else if _, ok := attrValue.GetValue().(*tracepb.AttributeValue_BoolValue); ok {
		if attrValue.GetBoolValue() {
			return "true", nil
		}
		return "false", nil
	} else if _, ok := attrValue.GetValue().(*tracepb.AttributeValue_IntValue); ok {
		return fmt.Sprintf("%d", attrValue.GetIntValue()), nil
	} else {
		return "", errors.New("cannot convert tag value")
	}
}

func convertTags(attributes *tracepb.Span_Attributes) (map[string]string, error) {
	tags := map[string]string{}
	for attrName, attr := range attributes.GetAttributeMap() {
		value, err := convertValue(attr)
		if err != nil {
			return tags, err
		}
		tags[attrName] = value
	}
	return tags, nil
}
