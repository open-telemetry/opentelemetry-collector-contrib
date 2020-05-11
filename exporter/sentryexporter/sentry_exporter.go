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

package sentryexporter

import (
	"context"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
)

// SentryExporter defines the Sentry Exporter
type SentryExporter struct {
	Dsn string
}

func (s *SentryExporter) pushTraceData(ctx context.Context, td pdata.Traces) (droppedSpans int, err error) {
	resourceSpans := td.ResourceSpans()

	if resourceSpans.Len() == 0 {
		return 0, nil
	}

	// Create a transaction for each resource
	// TODO: Create batches? Idk
	// transactions := make([]*SentryTransaction, 0, resourceSpans.Len())

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		if rs.IsNil() {
			continue
		}

		// TODO: Grab resource attributes to add to transaction
		// resource := rs.Resource() && check resource.IsNil() -> attr := resource.Attributes()

		// instrumentationLibrarySpans := rs.InstrumentationLibrarySpans()
	}

	// TODO: figure out how to send dropped spans
	return 0, nil
}

func spanToSentrySpan(span pdata.Span) (*SentrySpan, error) {
	if span.IsNil() {
		return nil, nil
	}

	traceID := span.TraceID().String()
	spanID := span.SpanID().String()
	parentSpanID := span.ParentSpanID().String()

	description := span.Name()
	// TODO: Generate Op
	op := ""

	tags := generateTagsFromAttributes(span.Attributes())

	endTimestamp := span.EndTime().String()
	timestamp := span.StartTime().String()

	status, message := generateStatusFromSpanStatus(span.Status())
	if message != "" {
		tags["status_message"] = message
	}

	tags["span_kind"] = span.Kind().String()

	sentrySpan := &SentrySpan{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Description:  description,
		Op:           op,
		Tags:         tags,
		EndTimestamp: endTimestamp,
		Timestamp:    timestamp,
		Status:       status,
	}

	return sentrySpan, nil
}

func generateTagsFromAttributes(attrs pdata.AttributeMap) Tags {
	tags := make(map[string]string)
	if attrs.Len() == 0 {
		return tags
	}

	attrs.ForEach(func(key string, attr pdata.AttributeValue) {
		switch attr.Type() {
		case pdata.AttributeValueSTRING:
			tags[key] = attr.StringVal()
		case pdata.AttributeValueBOOL:
			tags[key] = strconv.FormatBool(attr.BoolVal())
		case pdata.AttributeValueDOUBLE:
			tags[key] = strconv.FormatFloat(attr.DoubleVal(), 'g', -1, 64)
		case pdata.AttributeValueINT:
			tags[key] = strconv.FormatInt(attr.IntVal(), 10)
		}
	})

	return tags
}

func generateStatusFromSpanStatus(status pdata.SpanStatus) (string, string) {
	if status.IsNil() {
		return "unknown", ""
	}

	codes := [...]string{
		"ok",
		"cancelled",
		"unknown",
		"invalid_argument",
		"deadline_exceeded",
		"not_found",
		"already_exists",
		"permission_denied",
		"resource_exhausted",
		"failed_precondition",
		"aborted",
		"out_of_range",
		"unimplemented",
		"internal",
		"unavailable",
		"data_loss",
		"unauthenticated",
	}

	return codes[status.Code()], status.Message()
}

// CreateSentryExporter returns a new Sentry Exporter
func CreateSentryExporter(config *Config) (component.TraceExporter, error) {
	s := &SentryExporter{
		Dsn: config.Dsn,
	}

	exp, err := exporterhelper.NewTraceExporter(config, s.pushTraceData)

	return exp, err
}

// // Start the exporter
// func (s *SentryExporter) Start(ctx context.Context, host component.Host) error {}

// // ConsumeTraces receives pdata.Traces and sends them to Sentry
// func (s *SentryExporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {}

// // Shutdown the exporter
// func (s *SentryExporter) Shutdown(ctx context.Context) error {}
