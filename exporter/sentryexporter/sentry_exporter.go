// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	otelSentryExporterVersion = "0.0.2"
	otelSentryExporterName    = "sentry.opentelemetry"
)

// See OpenTelemetry span statuses in https://github.com/open-telemetry/opentelemetry-proto/blob/6cf77b2f544f6bc7fe1e4b4a8a52e5a42cb50ead/opentelemetry/proto/trace/v1/trace.proto#L303

// OpenTelemetry span status can be Unset, Ok, Error. HTTP and Grpc codes contained in tags can make it more detailed.

// canonicalCodesHTTPMap maps some HTTP codes to Sentry's span statuses. See possible mapping in https://develop.sentry.dev/sdk/event-payloads/span/
var canonicalCodesHTTPMap = map[string]sentry.SpanStatus{
	"400": sentry.SpanStatusFailedPrecondition, // SpanStatusInvalidArgument, SpanStatusOutOfRange
	"401": sentry.SpanStatusUnauthenticated,
	"403": sentry.SpanStatusPermissionDenied,
	"404": sentry.SpanStatusNotFound,
	"409": sentry.SpanStatusAborted, // SpanStatusAlreadyExists
	"429": sentry.SpanStatusResourceExhausted,
	"499": sentry.SpanStatusCanceled,
	"500": sentry.SpanStatusInternalError, // SpanStatusDataLoss, SpanStatusUnknown
	"501": sentry.SpanStatusUnimplemented,
	"503": sentry.SpanStatusUnavailable,
	"504": sentry.SpanStatusDeadlineExceeded,
}

// canonicalCodesGrpcMap maps some GRPC codes to Sentry's span statuses. See description in grpc documentation.
var canonicalCodesGrpcMap = map[string]sentry.SpanStatus{
	"1":  sentry.SpanStatusCanceled,
	"2":  sentry.SpanStatusUnknown,
	"3":  sentry.SpanStatusInvalidArgument,
	"4":  sentry.SpanStatusDeadlineExceeded,
	"5":  sentry.SpanStatusNotFound,
	"6":  sentry.SpanStatusAlreadyExists,
	"7":  sentry.SpanStatusPermissionDenied,
	"8":  sentry.SpanStatusResourceExhausted,
	"9":  sentry.SpanStatusFailedPrecondition,
	"10": sentry.SpanStatusAborted,
	"11": sentry.SpanStatusOutOfRange,
	"12": sentry.SpanStatusUnimplemented,
	"13": sentry.SpanStatusInternalError,
	"14": sentry.SpanStatusUnavailable,
	"15": sentry.SpanStatusDataLoss,
	"16": sentry.SpanStatusUnauthenticated,
}

// SentryExporter defines the Sentry Exporter.
type SentryExporter struct {
	transport   transport
	environment string
}

// pushTraceData takes an incoming OpenTelemetry trace, converts them into Sentry spans and transactions
// and sends them using Sentry's transport.
func (s *SentryExporter) pushTraceData(_ context.Context, td ptrace.Traces) error {
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

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			library := ils.Scope()

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
				if spanIsTransaction(otelSpan) {
					transactionMap[sentrySpan.SpanID] = transactionFromSpan(sentrySpan, s.environment)
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

	transactions := generateTransactions(transactionMap, orphanSpans, s.environment)

	transactions = append(transactions, exceptionEvents...)

	s.transport.SendEvents(transactions)

	return nil
}

// generateTransactions creates a set of Sentry transactions from a transaction map and orphan spans.
func generateTransactions(transactionMap map[sentry.SpanID]*sentry.Event, orphanSpans []*sentry.Span, environment string) []*sentry.Event {
	transactions := make([]*sentry.Event, 0, len(transactionMap)+len(orphanSpans))

	for _, t := range transactionMap {
		transactions = append(transactions, t)
	}

	for _, orphanSpan := range orphanSpans {
		t := transactionFromSpan(orphanSpan, environment)
		transactions = append(transactions, t)
	}

	return transactions
}

// convertEventsToSentryExceptions creates a set of sentry events from exception events present in spans.
// These events are stored in a mutated eventList
func convertEventsToSentryExceptions(eventList *[]*sentry.Event, events ptrace.SpanEventSlice, sentrySpan *sentry.Span) {
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		if event.Name() != "exception" {
			continue
		}
		var exceptionMessage, exceptionType string
		event.Attributes().Range(func(k string, v pcommon.Value) bool {
			switch k {
			case conventions.AttributeExceptionMessage:
				exceptionMessage = v.Str()
			case conventions.AttributeExceptionType:
				exceptionType = v.Str()
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
	event.EventID = generateEventID()

	event.Contexts["trace"] = sentry.TraceContext{
		TraceID:      span.TraceID,
		SpanID:       span.SpanID,
		ParentSpanID: span.ParentSpanID,
		Op:           span.Op,
		Description:  span.Description,
		Status:       span.Status,
	}.Map()

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

func convertToSentrySpan(span ptrace.Span, library pcommon.InstrumentationScope, resourceTags map[string]string) (sentrySpan *sentry.Span) {
	attributes := span.Attributes()
	name := span.Name()
	spanKind := span.Kind()

	op, description := generateSpanDescriptors(name, attributes, spanKind)
	tags := generateTagsFromAttributes(attributes)

	for k, v := range resourceTags {
		tags[k] = v
	}

	status, message := statusFromSpanStatus(span.Status(), tags)

	if message != "" {
		tags["status_message"] = message
	}

	if spanKind != ptrace.SpanKindUnspecified {
		tags["span_kind"] = traceutil.SpanKindStr(spanKind)
	}

	tags["library_name"] = library.Name()
	tags["library_version"] = library.Version()

	sentrySpan = &sentry.Span{
		TraceID:     sentry.TraceID(span.TraceID()),
		SpanID:      sentry.SpanID(span.SpanID()),
		Description: description,
		Op:          op,
		Tags:        tags,
		StartTime:   unixNanoToTime(span.StartTimestamp()),
		EndTime:     unixNanoToTime(span.EndTimestamp()),
		Status:      status,
	}

	if parentSpanID := span.ParentSpanID(); !parentSpanID.IsEmpty() {
		sentrySpan.ParentSpanID = sentry.SpanID(parentSpanID)
	}

	return sentrySpan
}

// generateSpanDescriptors generates generate span descriptors (op and description)
// from the name, attributes and SpanKind of an otel span based onSemantic Conventions
// described by the open telemetry specification.
//
// See https://github.com/open-telemetry/opentelemetry-specification/tree/5b78ee1/specification/trace/semantic_conventions
// for more details about the semantic conventions.
func generateSpanDescriptors(name string, attrs pcommon.Map, spanKind ptrace.SpanKind) (op string, description string) {
	var opBuilder strings.Builder
	var dBuilder strings.Builder

	// Generating span descriptors operates under the assumption that only one of the conventions are present.
	// In the possible case that multiple convention attributes are available, conventions are selected based
	// on what is most likely and what is most useful (ex. http is prioritized over FaaS)

	// If http.method exists, this is an http request span.
	if httpMethod, ok := attrs.Get(conventions.AttributeHTTPMethod); ok {
		opBuilder.WriteString("http")

		switch spanKind {
		case ptrace.SpanKindClient:
			opBuilder.WriteString(".client")
		case ptrace.SpanKindServer:
			opBuilder.WriteString(".server")
		case ptrace.SpanKindUnspecified:
		case ptrace.SpanKindInternal:
			opBuilder.WriteString(".internal")
		case ptrace.SpanKindProducer:
			opBuilder.WriteString(".producer")
		case ptrace.SpanKindConsumer:
			opBuilder.WriteString(".consumer")
		}

		// Ex. description="GET /api/users/{user_id}".
		fmt.Fprintf(&dBuilder, "%s %s", httpMethod.Str(), name)

		return opBuilder.String(), dBuilder.String()
	}

	// If db.type exists then this is a database call span.
	if _, ok := attrs.Get(conventions.AttributeDBSystem); ok {
		opBuilder.WriteString("db")

		// Use DB statement (Ex "SELECT * FROM table") if possible as description.
		if statement, okInst := attrs.Get(conventions.AttributeDBStatement); okInst {
			dBuilder.WriteString(statement.Str())
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
		opBuilder.WriteString(trigger.Str())

		return opBuilder.String(), name
	}

	// Default just use span.name.
	return "", name
}

func generateTagsFromResource(resource pcommon.Resource) map[string]string {
	return generateTagsFromAttributes(resource.Attributes())
}

func generateTagsFromAttributes(attrs pcommon.Map) map[string]string {
	tags := make(map[string]string)

	attrs.Range(func(key string, attr pcommon.Value) bool {
		switch attr.Type() {
		case pcommon.ValueTypeStr:
			tags[key] = attr.Str()
		case pcommon.ValueTypeBool:
			tags[key] = strconv.FormatBool(attr.Bool())
		case pcommon.ValueTypeDouble:
			tags[key] = strconv.FormatFloat(attr.Double(), 'g', -1, 64)
		case pcommon.ValueTypeInt:
			tags[key] = strconv.FormatInt(attr.Int(), 10)
		case pcommon.ValueTypeEmpty:
		case pcommon.ValueTypeMap:
		case pcommon.ValueTypeSlice:
		case pcommon.ValueTypeBytes:
		}
		return true
	})

	return tags
}

func statusFromSpanStatus(spanStatus ptrace.Status, tags map[string]string) (status sentry.SpanStatus, message string) {
	code := spanStatus.Code()
	if code < 0 || int(code) > 2 {
		return sentry.SpanStatusUnknown, fmt.Sprintf("error code %d", code)
	}
	httpCode, foundHTTPCode := tags["http.status_code"]
	grpcCode, foundGrpcCode := tags["rpc.grpc.status_code"]
	var sentryStatus sentry.SpanStatus
	switch {
	case code == 1 || code == 0:
		sentryStatus = sentry.SpanStatusOK
	case foundHTTPCode:
		httpStatus, foundHTTPStatus := canonicalCodesHTTPMap[httpCode]
		switch {
		case foundHTTPStatus:
			sentryStatus = httpStatus
		default:
			sentryStatus = sentry.SpanStatusUnknown
		}
	case foundGrpcCode:
		grpcStatus, foundGrpcStatus := canonicalCodesGrpcMap[grpcCode]
		switch {
		case foundGrpcStatus:
			sentryStatus = grpcStatus
		default:
			sentryStatus = sentry.SpanStatusUnknown
		}
	default:
		sentryStatus = sentry.SpanStatusUnknown
	}
	return sentryStatus, spanStatus.Message()
}

// spanIsTransaction determines if a span should be sent to Sentry as a transaction.
// If parent span id is empty or the span kind allows remote parent spans, then the span is a root span.
func spanIsTransaction(s ptrace.Span) bool {
	kind := s.Kind()
	return s.ParentSpanID() == pcommon.SpanID{} || kind == ptrace.SpanKindServer || kind == ptrace.SpanKindConsumer
}

// transactionFromSpan converts a span to a transaction.
func transactionFromSpan(span *sentry.Span, environment string) *sentry.Event {
	transaction := sentry.NewEvent()
	transaction.EventID = generateEventID()

	transaction.Contexts["trace"] = sentry.TraceContext{
		TraceID:      span.TraceID,
		SpanID:       span.SpanID,
		ParentSpanID: span.ParentSpanID,
		Op:           span.Op,
		Description:  span.Description,
		Status:       span.Status,
	}.Map()

	transaction.Type = "transaction"

	transaction.Sdk.Name = otelSentryExporterName
	transaction.Sdk.Version = otelSentryExporterVersion

	transaction.StartTime = span.StartTime
	transaction.Tags = span.Tags
	transaction.Timestamp = span.EndTime
	transaction.Transaction = span.Description
	if environment != "" {
		transaction.Environment = environment
	}

	return transaction
}

func uuid() string {
	id := make([]byte, 16)
	// Prefer rand.Read over rand.Reader, see https://go-review.googlesource.com/c/go/+/272326/.
	_, _ = rand.Read(id)
	id[6] &= 0x0F // clear version
	id[6] |= 0x40 // set version to 4 (random uuid)
	id[8] &= 0x3F // clear variant
	id[8] |= 0x80 // set to IETF variant
	return hex.EncodeToString(id)
}

func generateEventID() sentry.EventID {
	return sentry.EventID(uuid())
}

// createSentryExporter returns a new Sentry Exporter.
func createSentryExporter(config *Config, set exporter.CreateSettings) (exporter.Traces, error) {
	transport := newSentryTransport()

	clientOptions := sentry.ClientOptions{
		Dsn:         config.DSN,
		Environment: config.Environment,
	}

	if config.InsecureSkipVerify {
		clientOptions.HTTPTransport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	transport.Configure(clientOptions)

	s := &SentryExporter{
		transport:   transport,
		environment: config.Environment,
	}

	return exporterhelper.NewTracesExporter(
		context.TODO(),
		set,
		config,
		s.pushTraceData,
		exporterhelper.WithShutdown(func(ctx context.Context) error {
			allEventsFlushed := transport.Flush(ctx)

			if !allEventsFlushed {
				set.Logger.Warn("Could not flush all events, reached timeout")
			}

			return nil
		}),
	)
}
