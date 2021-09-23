// Copyright The OpenTelemetry Authors
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
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

const (
	otelSentryExporterVersion = "0.0.2"
	otelSentryExporterName    = "sentry.opentelemetry"
)

// canonicalCodes maps OpenTelemetry span codes to Sentry's span status.
// See numeric codes in https://github.com/open-telemetry/opentelemetry-proto/blob/6cf77b2f544f6bc7fe1e4b4a8a52e5a42cb50ead/opentelemetry/proto/trace/v1/trace.proto#L303
var canonicalCodes = [...]sentry.SpanStatus{
	sentry.SpanStatusUndefined,
	sentry.SpanStatusOK,
	sentry.SpanStatusUnknown,
}

// SentryExporter defines the Sentry Exporter.
type SentryExporter struct {
	transport transport
}

// pushTraceData takes an incoming OpenTelemetry trace, converts them into Sentry spans and transactions
// and sends them using Sentry's transport.
func (s *SentryExporter) pushTraceData(_ context.Context, td pdata.Traces) error {
	var exceptionEvents []*sentry.Event
	resourceSpans := td.ResourceSpans()
	if resourceSpans.Len() == 0 {
		return nil
	}

	maybeOrphanSpans := make([]*sentry.Span, 0, td.SpanCount())

	// Maps all child span ids to their root span.
	idMap := make(map[sentry.SpanID]sentry.SpanID)
	// Maps root span id to a transaction.
	transactionMap := make(map[sentry.SpanID]*sentry.Event)

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resourceTags := generateTagsFromResource(rs.Resource())

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			library := ils.InstrumentationLibrary()

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				otelSpan := spans.At(k)
				sentrySpan := convertToSentrySpan(otelSpan, library, resourceTags)
				convertEventsToSentryExceptions(&exceptionEvents, otelSpan.Events(), sentrySpan)

				// If a span is a root span, we consider it the start of a Sentry transaction.
				// We should then create a new transaction for that root span, and keep track of it.
				//
				// If the span is not a root span, we can either associate it with an existing
				// transaction, or we can temporarily consider it an orphan span.
				if isRootSpan(sentrySpan) {
					transactionMap[sentrySpan.SpanID] = transactionFromSpan(sentrySpan)
					idMap[sentrySpan.SpanID] = sentrySpan.SpanID
				} else {
					if rootSpanID, ok := idMap[sentrySpan.ParentSpanID]; ok {
						idMap[sentrySpan.SpanID] = rootSpanID
						transactionMap[rootSpanID].Spans = append(transactionMap[rootSpanID].Spans, sentrySpan)
					} else {
						maybeOrphanSpans = append(maybeOrphanSpans, sentrySpan)
					}
				}
			}
		}
	}

	if len(transactionMap) == 0 {
		return nil
	}

	// After the first pass through, we can't necessarily make the assumption we have not associated all
	// the spans with a transaction. As such, we must classify the remaining spans as orphans or not.
	orphanSpans := classifyAsOrphanSpans(maybeOrphanSpans, len(maybeOrphanSpans)+1, idMap, transactionMap)

	transactions := generateTransactions(transactionMap, orphanSpans)

	events := append(transactions, exceptionEvents...)

	s.transport.SendEvents(events)

	return nil
}

// generateTransactions creates a set of Sentry transactions from a transaction map and orphan spans.
func generateTransactions(transactionMap map[sentry.SpanID]*sentry.Event, orphanSpans []*sentry.Span) []*sentry.Event {
	transactions := make([]*sentry.Event, 0, len(transactionMap)+len(orphanSpans))

	for _, t := range transactionMap {
		transactions = append(transactions, t)
	}

	for _, orphanSpan := range orphanSpans {
		t := transactionFromSpan(orphanSpan)
		transactions = append(transactions, t)
	}

	return transactions
}

// convertEventsToSentryExceptions creates a set of sentry events from exception events present in spans.
// These events are stored in a mutated eventList
func convertEventsToSentryExceptions(eventList *[]*sentry.Event, events pdata.SpanEventSlice, sentrySpan *sentry.Span) {
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		if event.Name() != "exception" {
			continue
		}
		var exceptionMessage, exceptionType string
		event.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			switch k {
			case conventions.AttributeExceptionMessage:
				exceptionMessage = v.StringVal()
			case conventions.AttributeExceptionType:
				exceptionType = v.StringVal()
			}
			return true
		})
		if exceptionMessage == "" && exceptionType == "" {
			// `At least one of the following sets of attributes is required:
			// - exception.type
			// - exception.message`
			continue
		}
		sentryEvent, _ := sentryEventFromError(exceptionMessage, exceptionType, sentrySpan)
		*eventList = append(*eventList, sentryEvent)
	}
}

// sentryEventFromError creates a sentry event from error event in a span
func sentryEventFromError(errorMessage, errorType string, span *sentry.Span) (*sentry.Event, error) {
	if errorMessage == "" && errorType == "" {
		err := errors.New("error type and error message were both empty")
		return nil, err
	}
	event := sentry.NewEvent()

	event.Contexts["trace"] = sentry.TraceContext{
		TraceID: span.TraceID,
		SpanID:  span.SpanID,
		Op:      span.Op,
		Status:  span.Status,
	}

	event.Type = errorType
	event.Message = errorMessage
	event.Level = "error"
	event.Exception = []sentry.Exception{{
		Value: errorMessage,
		Type:  errorType,
	}}

	event.Sdk.Name = otelSentryExporterName
	event.Sdk.Version = otelSentryExporterVersion

	event.StartTime = span.StartTime
	event.Tags = span.Tags
	event.Timestamp = span.EndTime
	event.Transaction = span.Description

	return event, nil
}

// classifyAsOrphanSpans iterates through a list of possible orphan spans and tries to associate them
// with a transaction. As the order of the spans is not guaranteed, we have to recursively call
// classifyAsOrphanSpans to make sure that we did not leave any spans out of the transaction they belong to.
func classifyAsOrphanSpans(orphanSpans []*sentry.Span, prevLength int, idMap map[sentry.SpanID]sentry.SpanID, transactionMap map[sentry.SpanID]*sentry.Event) []*sentry.Span {
	if len(orphanSpans) == 0 || len(orphanSpans) == prevLength {
		return orphanSpans
	}

	newOrphanSpans := make([]*sentry.Span, 0, prevLength)

	for _, orphanSpan := range orphanSpans {
		if rootSpanID, ok := idMap[orphanSpan.ParentSpanID]; ok {
			idMap[orphanSpan.SpanID] = rootSpanID
			transactionMap[rootSpanID].Spans = append(transactionMap[rootSpanID].Spans, orphanSpan)
		} else {
			newOrphanSpans = append(newOrphanSpans, orphanSpan)
		}
	}

	return classifyAsOrphanSpans(newOrphanSpans, len(orphanSpans), idMap, transactionMap)
}

func convertToSentrySpan(span pdata.Span, library pdata.InstrumentationLibrary, resourceTags map[string]string) (sentrySpan *sentry.Span) {
	attributes := span.Attributes()
	name := span.Name()
	spanKind := span.Kind()

	op, description := generateSpanDescriptors(name, attributes, spanKind)
	tags := generateTagsFromAttributes(attributes)

	for k, v := range resourceTags {
		tags[k] = v
	}

	status, message := statusFromSpanStatus(span.Status())

	if message != "" {
		tags["status_message"] = message
	}

	if spanKind != pdata.SpanKindUnspecified {
		tags["span_kind"] = spanKind.String()
	}

	tags["library_name"] = library.Name()
	tags["library_version"] = library.Version()

	sentrySpan = &sentry.Span{
		TraceID:      span.TraceID().Bytes(),
		SpanID:       span.SpanID().Bytes(),
		ParentSpanID: span.ParentSpanID().Bytes(),
		Description:  description,
		Op:           op,
		Tags:         tags,
		StartTime:    unixNanoToTime(span.StartTimestamp()),
		EndTime:      unixNanoToTime(span.EndTimestamp()),
		Status:       status,
	}

	return sentrySpan
}

// generateSpanDescriptors generates generate span descriptors (op and description)
// from the name, attributes and SpanKind of an otel span based onSemantic Conventions
// described by the open telemetry specification.
//
// See https://github.com/open-telemetry/opentelemetry-specification/tree/5b78ee1/specification/trace/semantic_conventions
// for more details about the semantic conventions.
func generateSpanDescriptors(name string, attrs pdata.AttributeMap, spanKind pdata.SpanKind) (op string, description string) {
	var opBuilder strings.Builder
	var dBuilder strings.Builder

	// Generating span descriptors operates under the assumption that only one of the conventions are present.
	// In the possible case that multiple convention attributes are available, conventions are selected based
	// on what is most likely and what is most useful (ex. http is prioritized over FaaS)

	// If http.method exists, this is an http request span.
	if httpMethod, ok := attrs.Get(conventions.AttributeHTTPMethod); ok {
		opBuilder.WriteString("http")

		switch spanKind {
		case pdata.SpanKindClient:
			opBuilder.WriteString(".client")
		case pdata.SpanKindServer:
			opBuilder.WriteString(".server")
		}

		// Ex. description="GET /api/users/{user_id}".
		fmt.Fprintf(&dBuilder, "%s %s", httpMethod.StringVal(), name)

		return opBuilder.String(), dBuilder.String()
	}

	// If db.type exists then this is a database call span.
	if _, ok := attrs.Get(conventions.AttributeDBSystem); ok {
		opBuilder.WriteString("db")

		// Use DB statement (Ex "SELECT * FROM table") if possible as description.
		if statement, okInst := attrs.Get(conventions.AttributeDBStatement); okInst {
			dBuilder.WriteString(statement.StringVal())
		} else {
			dBuilder.WriteString(name)
		}

		return opBuilder.String(), dBuilder.String()
	}

	// If rpc.service exists then this is a rpc call span.
	if _, ok := attrs.Get(conventions.AttributeRPCService); ok {
		opBuilder.WriteString("rpc")

		return opBuilder.String(), name
	}

	// If messaging.system exists then this is a messaging system span.
	if _, ok := attrs.Get("messaging.system"); ok {
		opBuilder.WriteString("message")

		return opBuilder.String(), name
	}

	// If faas.trigger exists then this is a function as a service span.
	if trigger, ok := attrs.Get("faas.trigger"); ok {
		opBuilder.WriteString(trigger.StringVal())

		return opBuilder.String(), name
	}

	// Default just use span.name.
	return "", name
}

func generateTagsFromResource(resource pdata.Resource) map[string]string {
	return generateTagsFromAttributes(resource.Attributes())
}

func generateTagsFromAttributes(attrs pdata.AttributeMap) map[string]string {
	tags := make(map[string]string)

	attrs.Range(func(key string, attr pdata.AttributeValue) bool {
		switch attr.Type() {
		case pdata.AttributeValueTypeString:
			tags[key] = attr.StringVal()
		case pdata.AttributeValueTypeBool:
			tags[key] = strconv.FormatBool(attr.BoolVal())
		case pdata.AttributeValueTypeDouble:
			tags[key] = strconv.FormatFloat(attr.DoubleVal(), 'g', -1, 64)
		case pdata.AttributeValueTypeInt:
			tags[key] = strconv.FormatInt(attr.IntVal(), 10)
		}
		return true
	})

	return tags
}

func statusFromSpanStatus(spanStatus pdata.SpanStatus) (status sentry.SpanStatus, message string) {
	code := spanStatus.Code()
	if code < 0 || int(code) >= len(canonicalCodes) {
		return sentry.SpanStatusUnknown, fmt.Sprintf("error code %d", code)
	}

	return canonicalCodes[code], spanStatus.Message()
}

// isRootSpan determines if a span is a root span.
// If parent span id is empty, then the span is a root span.
func isRootSpan(s *sentry.Span) bool {
	return s.ParentSpanID == sentry.SpanID{}
}

// transactionFromSpan converts a span to a transaction.
func transactionFromSpan(span *sentry.Span) *sentry.Event {
	transaction := sentry.NewEvent()

	transaction.Contexts["trace"] = sentry.TraceContext{
		TraceID: span.TraceID,
		SpanID:  span.SpanID,
		Op:      span.Op,
		Status:  span.Status,
	}

	transaction.Type = "transaction"

	transaction.Sdk.Name = otelSentryExporterName
	transaction.Sdk.Version = otelSentryExporterVersion

	transaction.StartTime = span.StartTime
	transaction.Tags = span.Tags
	transaction.Timestamp = span.EndTime
	transaction.Transaction = span.Description

	return transaction
}

// CreateSentryExporter returns a new Sentry Exporter.
func CreateSentryExporter(config *Config, set component.ExporterCreateSettings) (component.TracesExporter, error) {
	transport := newSentryTransport()

	clientOptions := sentry.ClientOptions{
		Dsn: config.DSN,
	}

	if config.InsecureSkipVerify {
		clientOptions.HTTPTransport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	transport.Configure(clientOptions)

	s := &SentryExporter{
		transport: transport,
	}

	return exporterhelper.NewTracesExporter(
		config,
		set,
		s.pushTraceData,
		exporterhelper.WithShutdown(func(ctx context.Context) error {
			allEventsFlushed := transport.Flush(ctx)

			if !allEventsFlushed {
				log.Print("Could not flush all events, reached timeout")
			}

			return nil
		}),
	)
}
