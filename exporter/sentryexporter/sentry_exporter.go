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
	"strings"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"github.com/open-telemetry/opentelemetry-collector/internal"
	"github.com/open-telemetry/opentelemetry-collector/translator/conventions"
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

		// tags := generateTagsFromAttributes(rs.Resource().Attributes())

		// TODO: Grab resource attributes to add to transaction
		// resource := rs.Resource() && check attr := resource.Attributes()

		instLibSpans := rs.InstrumentationLibrarySpans()

		for j := 0; j < instLibSpans.Len(); j++ {
			// Each ils represents a transaction
			ils := instLibSpans.At(j)

			// TODO: Add ils.InstrumentationLibrary().Name() and ils.InstrumentationLibrary().Version()
			if ils.IsNil() {
				continue
			}
		}
	}

	// TODO: figure out how to send dropped spans
	return 0, nil
}

// TODO: Span.Link
// TODO; Span.Event -> create breadcrumbs
func spanToSentrySpan(span pdata.Span) (*SentrySpan, bool) {
	if span.IsNil() {
		return nil, false
	}

	traceID := span.TraceID().String()
	spanID := span.SpanID().String()
	parentSpanID := span.ParentSpanID().String()

	// If parent span id is empty, this span is a root span
	// See: https://github.com/open-telemetry/opentelemetry-proto/blob/master/opentelemetry/proto/trace/v1/trace.proto#L82
	isRootSpan := parentSpanID == ""

	attributes := span.Attributes()
	name := span.Name()
	spanKind := span.Kind()

	op, description := generateSpanDescriptors(name, attributes, spanKind)
	tags := generateTagsFromAttributes(attributes)

	endTimestamp := internal.UnixNanoToTime(span.EndTime())
	timestamp := internal.UnixNanoToTime(span.StartTime())

	status, message := generateStatusFromSpanStatus(span.Status())
	if message != "" {
		tags["status_message"] = message
	}

	if spanKind != pdata.SpanKindUNSPECIFIED {
		tags["span_kind"] = spanKind.String()
	}

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

	return sentrySpan, isRootSpan
}

// We use Semantic Conventions described by the open telemetry specification to
// decide what op and description to apply to the span
// https://github.com/open-telemetry/opentelemetry-specification/tree/master/specification/trace/semantic_conventions
func generateSpanDescriptors(name string, attrs pdata.AttributeMap, spanKind pdata.SpanKind) (op string, description string) {
	var opString strings.Builder
	var dString strings.Builder

	// if http.method exists, this is an http request span
	if httpMethod, ok := attrs.Get(conventions.AttributeHTTPMethod); ok {
		opString.WriteString("http")

		switch spanKind {
		case pdata.SpanKindCLIENT:
			opString.WriteString(".client")
		case pdata.SpanKindSERVER:
			opString.WriteString(".server")
		}

		// Ex. description="GET /api/users/{user_id}"
		dString.WriteString(httpMethod.StringVal())
		dString.WriteString(" ")
		dString.WriteString(name)

		return opString.String(), dString.String()
	}

	// if db.type exists then this is a database call span
	if _, ok := attrs.Get(conventions.AttributeDBType); ok {
		opString.WriteString("db")

		// Use DB statement (Ex "SELECT * FROM table") if possible as description
		if statement, okInst := attrs.Get(conventions.AttributeDBStatement); okInst {
			dString.WriteString(statement.StringVal())
		} else {
			dString.WriteString(name)
		}

		return opString.String(), dString.String()
	}

	// if rpc.service exists then this is a rpc call span
	if _, ok := attrs.Get(conventions.AttributeRPCService); ok {
		opString.WriteString("rpc")

		return opString.String(), name
	}

	// if messaging.system exists then this is a messaging system span
	if _, ok := attrs.Get("messaging.system"); ok {
		opString.WriteString("message")

		return opString.String(), name
	}

	// if faas.trigger exists then this is a function as a service span
	if trigger, ok := attrs.Get("faas.trigger"); ok {
		opString.WriteString(trigger.StringVal())

		return opString.String(), name
	}

	// Default just use span.name
	return "", name
}

func generateTagsFromAttributes(attrs pdata.AttributeMap) Tags {
	tags := make(map[string]string)

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
