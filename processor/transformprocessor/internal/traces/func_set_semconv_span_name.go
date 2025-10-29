// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

type setSemconvSpanNameArguments struct {
	SemconvVersion            string
	OriginalSpanNameAttribute ottl.Optional[string]
}

func NewSetSemconvSpanNameFactory() ottl.Factory[ottlspan.TransformContext] {
	return ottl.NewFactory("set_semconv_span_name", &setSemconvSpanNameArguments{}, createSetSemconvSpanNameFunction)
}

func createSetSemconvSpanNameFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlspan.TransformContext], error) {

	args, ok := oArgs.(*setSemconvSpanNameArguments)

	if !ok {
		return nil, errors.New("NewSetSemconvSpanNameFactory args must be of type *setSemconvSpanNameArguments")
	}
	var r error = nil
	return func(_ context.Context, tCtx ottlspan.TransformContext) (any, error) {
		return setSemconvSpanName(args.SemconvVersion, args.OriginalSpanNameAttribute, tCtx.GetSpan()), nil
	}, r
}

func setSemconvSpanName(semconvVersion string, originalSpanNameAttribute ottl.Optional[string], span ptrace.Span) error {
	if semconvVersion != "1.37.0" {
		// Currently only v1.37.0 is supported
		return fmt.Errorf("unsupported semconv version: %s", semconvVersion)
	}
	originalSpanName := span.Name()
	semConvSpanName := SemconvSpanName(span)
	span.SetName(semConvSpanName)
	if originalSpanName != semConvSpanName && originalSpanNameAttribute.GetOr("") != "" {
		span.Attributes().PutStr(originalSpanNameAttribute.Get(), originalSpanName)
	}
	return nil
}

func SemconvSpanName(span ptrace.Span) string {
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
		return system.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/database/database-spans/
func dbSpanName(span ptrace.Span) string {
	if system, ok := attributeValue(span, semconv.DBSystemNameKey, "db.system"); ok {
		if summary, ok := span.Attributes().Get(string(semconv.DBQuerySummaryKey)); ok {
			return summary.AsString()
		}
		operationName := ""
		if operation, ok := attributeValue(span, semconv.DBOperationNameKey, "db.operation"); ok {
			operationName = operation.AsString()
		}

		target := databaseTarget(span)

		if operationName != "" && target != "" {
			return operationName + " " + target
		}
		if operationName != "" {
			return operationName
		}
		if target != "" {
			return target
		}
		return system.AsString()
	}
	return ""
}

func databaseTarget(span ptrace.Span) string {
	dbNamespace := ""
	if namespace, ok := attributeValue(span, semconv.DBNamespaceKey, "db.name"); ok {
		dbNamespace = namespace.AsString()
	}

	if collection, ok := span.Attributes().Get(string(semconv.DBCollectionNameKey)); ok {
		if dbNamespace != "" {
			return dbNamespace + "." + collection.AsString()
		}
		return collection.AsString()
	}

	if storedProcedure, ok := span.Attributes().Get(string(semconv.DBStoredProcedureNameKey)); ok {
		if dbNamespace != "" {
			return dbNamespace + "." + storedProcedure.AsString()
		}
		return storedProcedure.AsString()
	}

	if dbNamespace != "" {
		return dbNamespace
	}

	if serverAddress, ok := span.Attributes().Get(string(semconv.ServerAddressKey)); ok {
		if serverPort, ok := span.Attributes().Get(string(semconv.ServerPortKey)); ok {
			return serverAddress.AsString() + ":" + serverPort.AsString()
		}
		return serverAddress.AsString()
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
		return system.AsString()
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
