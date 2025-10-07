// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.37.0"
)

func SemConvSpanName(span ptrace.Span) string {
	switch span.Kind() {
	case ptrace.SpanKindServer:
		spanName := httpSpanName(span, semconv.HTTPRouteKey)
		if spanName != "" {
			return spanName
		}
		spanName = rpcSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindConsumer:
		spanName := messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindClient:
		spanName := httpSpanName(span, semconv.URLTemplateKey)
		if spanName != "" {
			return spanName
		}
		spanName = rpcSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = dbSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindProducer:
		spanName := messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	}
	// If no semantic convention defines the span name, default to the original span name
	return span.Name()
}

// https://opentelemetry.io/docs/specs/semconv/http/http-spans/
func httpSpanName(span ptrace.Span, subject attribute.Key) string {
	if method, ok := attributeValue(span, semconv.HTTPRequestMethodKey, "http.method"); ok {
		if subjectVal, ok := span.Attributes().Get(string(subject)); ok {
			return method.AsString() + " " + subjectVal.AsString()
		}
		return method.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/rpc/rpc-spans/
func rpcSpanName(span ptrace.Span) string {
	if system, ok := span.Attributes().Get(string(semconv.RPCSystemKey)); ok {
		method, okMethod := attributeValue(span, semconv.RPCMethodKey, "rpc.grpc.method")
		service, okService := attributeValue(span, semconv.RPCServiceKey, "rpc.grpc.service")

		if okMethod && okService {
			return service.AsString() + "/" + method.AsString()
		}
		if okMethod {
			return method.AsString()
		}
		if okService {
			return service.AsString() + "/*"
		}
		return "(" + system.AsString() + ")"
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/database/database-spans/
func dbSpanName(span ptrace.Span) string {
	if system, ok := attributeValue(span, semconv.DBSystemNameKey, "db.system"); ok {
		spanName := ""
		if operation, ok := attributeValue(span, semconv.DBOperationNameKey, "db.operation"); ok {
			spanName += operation.AsString() + " "
		}
		if namespace, ok := span.Attributes().Get(string(semconv.DBNamespaceKey)); ok {
			spanName += namespace.AsString() + "."
		}
		if collection, ok := attributeValue(span, semconv.DBCollectionNameKey, "db.name"); ok {
			spanName += collection.AsString()
		}
		if spanName == "" {
			return "(" + system.AsString() + ")"
		}
		return spanName
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/#span-name
func messagingSpanName(span ptrace.Span) string {
	if system, ok := span.Attributes().Get(string(semconv.MessagingSystemKey)); ok {
		operation, okOperation := attributeValue(span, semconv.MessagingOperationNameKey, "messaging.operation")
		destination := messagingDestination(span)

		if okOperation && destination != "" {
			return operation.AsString() + " " + destination
		}
		if destination != "" {
			return destination
		}
		if okOperation {
			return operation.AsString()
		}
		return "(" + system.AsString() + ")"
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/#span-name
func messagingDestination(span ptrace.Span) string {
	if temporaryDestination, ok := span.Attributes().Get(string(semconv.MessagingDestinationTemporaryKey)); ok {
		if temporaryDestination.Bool() {
			return "(temporary)"
		}
	}
	if anonymousDestination, ok := span.Attributes().Get(string(semconv.MessagingDestinationAnonymousKey)); ok {
		// anonymous destinations should also be marked as temporary by the messaging instrumentation
		// double check in case the instrumentation forgot to mark the anonymous destination as temporary
		if anonymousDestination.Bool() {
			return "(anonymous)"
		}
	}
	if destinationTemplate, ok := span.Attributes().Get(string(semconv.MessagingDestinationTemplateKey)); ok {
		return destinationTemplate.AsString()
	}
	if destinationName, ok := attributeValue(span, semconv.MessagingDestinationNameKey, "messaging.destination"); ok {
		return destinationName.AsString()
	}
	if serverAddress, ok := span.Attributes().Get(string(semconv.ServerAddressKey)); ok {
		if serverPort, ok := span.Attributes().Get(string(semconv.ServerPortKey)); ok {
			return serverAddress.AsString() + ":" + serverPort.AsString()
		}
		return serverAddress.AsString()
	}
	return ""
}

func attributeValue(span ptrace.Span, name attribute.Key, alias string) (pcommon.Value, bool) {
	if val, ok := span.Attributes().Get(string(name)); ok {
		return val, true
	}
	if alias != "" {
		if val, ok := span.Attributes().Get(alias); ok {
			return val, true
		}
	}
	return pcommon.Value{}, false
}
